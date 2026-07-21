package rules

import "github.com/hyturing/compose-pulse/internal/diagnosis/engine"

// DefaultRules returns the Phase 2 high-confidence rule set in registration order.
func DefaultRules() []engine.Rule {
	return []engine.Rule{
		missingEnvVarRule{},
		invalidComposeRule{},
		hostPortOccupiedRule{},
		containerNameConflictRule{},
		bindSourceMissingRule{},
		imagePullDeniedRule{},
		imageManifestMissingRule{},
		imagePlatformMismatchRule{},
		exit126Rule{},
		exit127Rule{},
		oomKilledRule{},
		healthMissingExecutableRule{},
		localhostInContainerRule{},
		dependsOnStartedRaceRule{},
		resourceOOMRule{},
		identicalExitLoopRule{},
		probeTCPRefusedRule{},
	}
}
