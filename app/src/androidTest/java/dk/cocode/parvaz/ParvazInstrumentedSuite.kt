package dk.cocode.parvaz

import dk.cocode.parvaz.mitm.CaExporterInstrumentedTest
import dk.cocode.parvaz.mitm.CaInstallerInstrumentedTest
import dk.cocode.parvaz.settings.ParvazSettingsInstrumentedTest
import dk.cocode.parvaz.vpn.CoreLauncherInstrumentedTest
import org.junit.runner.RunWith
import org.junit.runners.Suite

/**
 * Single entry point for every instrumented test in the app module.
 *
 * Run the whole suite from the command line with:
 *
 *     ./gradlew :app:connectedDebugAndroidTest \
 *         -Pandroid.testInstrumentationRunnerArguments.class=dk.cocode.parvaz.ParvazInstrumentedSuite
 *
 * (or just `./gradlew :app:connectedDebugAndroidTest` to discover all
 *  @RunWith-annotated classes — but the suite gives you a stable name
 *  to reference from CI configs and IDE run-configurations.)
 *
 * When you add a new instrumented test class, list it here too. The
 * cost is one line; the benefit is you can ALWAYS reproduce the full
 * coverage from a single command, and CI never silently skips a class
 * because of a discovery quirk.
 *
 * ExampleInstrumentedTest and CaStoreDumpTest are intentionally excluded:
 * the first is template scaffolding, the second is a manual diagnostic
 * that logs device CA-store state rather than asserting product behavior.
 */
@RunWith(Suite::class)
@Suite.SuiteClasses(
    CaExporterInstrumentedTest::class,
    CaInstallerInstrumentedTest::class,
    CoreLauncherInstrumentedTest::class,
    MainActivityDeepLinkRecreateTest::class,
    MainActivityStaleOnboardingTest::class,
    ParvazSettingsInstrumentedTest::class,
)
class ParvazInstrumentedSuite
