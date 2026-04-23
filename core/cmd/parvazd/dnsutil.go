package main

import "encoding/binary"

// DNS constants we care about. Kept tight — we only read QTYPE on
// incoming queries and rewrite flags on synthesised responses.
const (
	dnsTypeAAAA    uint16 = 28
	dnsHeaderBytes        = 12

	// DNS flag bits (see RFC 1035 §4.1.1). Byte 2 holds QR|Opcode|AA|TC|RD;
	// byte 3 holds RA|Z|RCODE. We only set QR/RA and strip AA/TC/Z/RCODE —
	// the opcode and RD bits are echoed from the query.
	//
	// byte2Mask = QR | Opcode(4) | RD   (preserved/set)
	// byte2Mask retains the opcode + RD bits from the query and guarantees
	// QR=1 by ORing 0x80 separately.
	byte2Preserve byte = 0x79 // 0b01111001 — opcode nibble + RD, clears AA+TC
	rcodeNoError  byte = 0x00
	rcodeServFail byte = 0x02
)

// parseQueryQType returns the QTYPE of the first question in a DNS
// wire-format query. Returns (0, false) for anything malformed — the
// caller should treat that as "type unknown" and forward without
// local synthesis.
func parseQueryQType(query []byte) (uint16, bool) {
	if len(query) < dnsHeaderBytes {
		return 0, false
	}
	qdcount := binary.BigEndian.Uint16(query[4:6])
	if qdcount == 0 {
		return 0, false
	}
	off := dnsHeaderBytes
	for off < len(query) {
		l := int(query[off])
		if l == 0 {
			off++
			break
		}
		// Compression pointers (0xC0+) are illegal in a question's QNAME,
		// so treat them as malformed rather than chasing the pointer.
		if l&0xC0 != 0 {
			return 0, false
		}
		off += 1 + l
	}
	if off+4 > len(query) {
		return 0, false
	}
	return binary.BigEndian.Uint16(query[off : off+2]), true
}

// synthesizeEmpty returns a DNS response mirroring `query` with QR=1,
// RA=1, RCODE=NOERROR, and zero answer/authority/additional records.
// Used to short-circuit AAAA queries while the TUN remains IPv4-only
// — the client sees a valid "no such record" answer immediately.
func synthesizeEmpty(query []byte) []byte {
	return synthesizeResponse(query, rcodeNoError)
}

// synthesizeSERVFAIL builds the same response shape with RCODE=2.
// Used when the upstream DoH fetch fails — without this the resolver
// waits out its retry budget (often several seconds) before giving
// up, making every page load hang during upstream outages.
func synthesizeSERVFAIL(query []byte) []byte {
	return synthesizeResponse(query, rcodeServFail)
}

// synthesizeResponse copies the query's header + question section,
// sets response flags, and zeros the counts past QDCOUNT. The
// question is carried back for resolver pedantry — some clients
// validate it matches what they asked.
func synthesizeResponse(query []byte, rcode byte) []byte {
	if len(query) < dnsHeaderBytes {
		return nil
	}
	resp := make([]byte, len(query))
	copy(resp, query)
	resp[2] = (query[2] & byte2Preserve) | 0x80 // QR=1, clear AA+TC, keep opcode+RD
	resp[3] = 0x80 | (rcode & 0x0F)             // RA=1, clear Z, set RCODE
	// ANCOUNT, NSCOUNT, ARCOUNT = 0 (QDCOUNT echoed from query).
	resp[6], resp[7] = 0, 0
	resp[8], resp[9] = 0, 0
	resp[10], resp[11] = 0, 0
	return resp
}
