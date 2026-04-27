package dk.cocode.parvaz.update

/**
 * Semver triple used for "current vs. latest GitHub release" comparison.
 * GitHub tags are formatted `v0.1.4`; the leading `v` is optional.
 * Pre-release suffixes (`-rc.1`, `-alpha`) are stripped — we only ship
 * stable releases through this updater, and stripping keeps `compareTo`
 * total without pulling in a full semver library.
 */
data class Version(val major: Int, val minor: Int, val patch: Int) : Comparable<Version> {

    override fun compareTo(other: Version): Int {
        if (major != other.major) return major - other.major
        if (minor != other.minor) return minor - other.minor
        return patch - other.patch
    }

    fun isNewerThan(other: Version): Boolean = this > other

    companion object {
        private val PATTERN = Regex("""^v?(\d+)\.(\d+)(?:\.(\d+))?""")

        fun parse(raw: String): Version? {
            val trimmed = raw.trim()
            val match = PATTERN.find(trimmed) ?: return null
            val (maj, min, patch) = match.destructured
            return Version(
                major = maj.toIntOrNull() ?: return null,
                minor = min.toIntOrNull() ?: return null,
                patch = patch.toIntOrNull() ?: 0,
            )
        }
    }
}
