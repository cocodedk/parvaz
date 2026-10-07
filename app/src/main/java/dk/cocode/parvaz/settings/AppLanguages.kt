package dk.cocode.parvaz.settings

/**
 * The languages a user can pick in the app, in the order the language
 * picker lists them. The first is the default. Each code is a language
 * tag that `MainActivity.attachBaseContext` turns into a locale, so a
 * new one only needs a `res/values-<code>/` folder.
 */
object AppLanguages {
    val all = listOf("fa", "en", "da")

    /** The language after [current], wrapping round; the default for an unknown code. */
    fun next(current: String): String = all[(all.indexOf(current) + 1) % all.size]
}
