# Parvaz — visual identity

One mark, one palette, used across launcher icon, cold-launch splash,
in-app splash, website favicon, and GitHub social preview.

## The mark

A paper-plane silhouette inside a perforated oxblood postal-stamp ring.

- **Plane** — asymmetric kite tilted up-right. Inherited from the
  original website favicon (`website/favicon.svg`, pre-M10b) so the
  flight metaphor carries through. Filled oxblood; inner fold accent
  drawn in deeper oxblood for paper-grain depth.
- **Stamp ring** — oxblood circle stroke with 12 evenly-spaced Paper
  perforation notches. Reads as "official document / cleared for
  flight" in line with the broader NOTAM aesthetic.
- **Background inside the ring** — Paper. The mark sits on Paper so it
  reads identically against the app's Paper backgrounds and against the
  Android launcher's user-chosen wallpaper.

Master file: [`identity/parvaz-mark.svg`](identity/parvaz-mark.svg).
ViewBox `0 0 100 100`. Crisp from 16x16 (favicon) up.

## Palette

Reused verbatim from `app/src/main/res/values/colors.xml` and
`ui/theme/Color.kt` — kept in sync there.

| Role | Hex | Use |
|---|---|---|
| Paper | `#F1E8D4` | Stamp inset; launcher background |
| Oxblood | `#A8361C` | Stamp ring; plane fill |
| Oxblood deep | `#7A2614` | Inner fold accent on the plane |

## Usage

- **Adaptive launcher icon** — foreground = plane only (no ring); the
  Android launcher's mask provides the circle/squircle shape, so the
  ring would get cropped unpredictably. Background = solid Paper.
  Monochrome variant (Android 13+ themed icons) = plane silhouette in
  pure black on transparent.
- **Cold-launch splash (Android 12+ SplashScreen API)** — reuse the
  same launcher foreground (plane-only); the system's splash icon
  container provides the surrounding Paper circle. Splash background
  colour = Paper, so the launcher → splash → app transition is one
  continuous Paper field with the plane appearing to land.
- **In-app splash (`SplashScreen.kt`)** — render the **full mark**
  (plane + ring) above the `پرواز` wordmark so the lockup picks up the
  rubber-stamp framing the cold splash deliberately omits.
- **Website + favicon** — the full mark replaces
  `website/favicon.svg`. Same SVG, no Android-specific stripping.
- **GitHub social preview** — manual upload step in repo settings.
  Use a 1280x640 PNG export of the full mark on Paper background. Not
  scriptable; do it once after merging M10b.

## Why this combination

The NOTAM identity already leans hard on rubber stamps (rubber-stamp
button states, oxblood error overlays). A stamped mark is the natural
extension at the brand layer. The paper-plane inside the stamp keeps
the literal `parvaz / flight` semantics; without it the mark would
read as generic officialdom. Using just the plane for the launcher
icon (and trusting the system mask) avoids the double-framing problem
where two circles compete inside the same masked tile.
