package dk.cocode.parvaz.mitm

import java.io.ByteArrayInputStream
import java.security.MessageDigest
import java.security.cert.CertificateFactory
import java.security.cert.X509Certificate

/**
 * Pure helpers for turning the PEM blob parvazd -gen-ca writes into the
 * DER bytes Android's KeyChain wants, and for computing the SHA-256
 * identity we compare against when walking AndroidCAStore after the
 * system install intent returns.
 *
 * No Android dependencies — Java's CertificateFactory + MessageDigest
 * work on the host JVM too, so this is covered by a plain
 * `./gradlew test` run.
 */
object CaFingerprint {

    /**
     * Parse a PEM-encoded X.509 certificate and return its DER bytes.
     * Throws if the input isn't a single valid certificate.
     */
    fun pemToDer(pem: ByteArray): ByteArray {
        val factory = CertificateFactory.getInstance("X.509")
        val cert = factory.generateCertificate(ByteArrayInputStream(pem)) as X509Certificate
        return cert.encoded
    }

    /** SHA-256 of arbitrary bytes — used to identify a CA by fingerprint. */
    fun sha256(bytes: ByteArray): ByteArray =
        MessageDigest.getInstance("SHA-256").digest(bytes)

    /** Convenience: PEM → DER → SHA-256. */
    fun fingerprint(pem: ByteArray): ByteArray = sha256(pemToDer(pem))
}
