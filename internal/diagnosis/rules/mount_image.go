package rules

import (
	"github.com/hyturing/compose-pulse/internal/diagnosis/confidence"
	"github.com/hyturing/compose-pulse/internal/diagnosis/engine"
	"github.com/hyturing/compose-pulse/internal/diagnosis/evidence"
	"github.com/hyturing/compose-pulse/internal/model"
)

type bindSourceMissingRule struct{}

func (bindSourceMissingRule) ID() string { return "mount.bind_source_missing" }
func (bindSourceMissingRule) Description() string {
	return "Bind mount source path does not exist on the host"
}

func (bindSourceMissingRule) Evaluate(ctx *engine.RunContext) []model.Finding {
	evs := eventsMatching(ctx, func(ev model.Event) bool {
		if dataString(ev, "error_kind") == "bind_source_missing" {
			return true
		}
		return containsAny(ev.Message,
			"bind source path does not exist",
			"not a directory",
			"no such file or directory",
		) && containsAny(ev.Message, "mount", "bind", "volume")
	})
	if len(evs) == 0 {
		return nil
	}
	ev := evs[0]
	src := dataString(ev, "bind_source")
	root := "bind mount source path does not exist"
	if src != "" {
		root = "bind mount source path does not exist: " + src
	}
	return []model.Finding{{
		RuleID:     "mount.bind_source_missing",
		Service:    ev.Service,
		RootCause:  root,
		Evidence:   []string{evidence.Line("compose_error", ev.Message), evidence.KV("bind_source", src)},
		Confidence: confidence.High,
		SuggestedFixes: []string{
			"Create the host path before starting, or correct the bind-mount source in the compose file",
		},
	}}
}

type imagePullDeniedRule struct{}

func (imagePullDeniedRule) ID() string { return "image.pull_denied" }
func (imagePullDeniedRule) Description() string {
	return "Image pull failed due to auth/access denied"
}

func (imagePullDeniedRule) Evaluate(ctx *engine.RunContext) []model.Finding {
	evs := eventsMatching(ctx, func(ev model.Event) bool {
		if dataString(ev, "progress_kind") == "auth_error" {
			return true
		}
		return containsAny(ev.Message,
			"pull access denied",
			"authentication required",
			"unauthorized",
			"requested access to the resource is denied",
		)
	})
	if len(evs) == 0 {
		return nil
	}
	ev := evs[0]
	image := dataString(ev, "image")
	root := "image pull access denied"
	if image != "" {
		root = "image pull access denied for " + image
	}
	return []model.Finding{{
		RuleID:     "image.pull_denied",
		Service:    ev.Service,
		RootCause:  root,
		Evidence:   []string{evidence.Line("compose_error", ev.Message), evidence.KV("image", image)},
		Confidence: confidence.High,
		SuggestedFixes: []string{
			"docker login to the registry, or use an image/tag the current credentials can pull",
		},
	}}
}

type imageManifestMissingRule struct{}

func (imageManifestMissingRule) ID() string { return "image.manifest_missing" }
func (imageManifestMissingRule) Description() string {
	return "Image manifest/tag was not found in the registry"
}

func (imageManifestMissingRule) Evaluate(ctx *engine.RunContext) []model.Finding {
	evs := eventsMatching(ctx, func(ev model.Event) bool {
		if dataString(ev, "progress_kind") == "manifest_missing" {
			return true
		}
		if containsAny(ev.Message, "pull access denied", "unauthorized", "authentication required") {
			return false
		}
		return containsAny(ev.Message, "manifest unknown", "manifest for") && containsAny(ev.Message, "not found", "unknown")
	})
	if len(evs) == 0 {
		return nil
	}
	ev := evs[0]
	image := dataString(ev, "image")
	root := "image manifest not found"
	if image != "" {
		root = "image manifest not found for " + image
	}
	return []model.Finding{{
		RuleID:     "image.manifest_missing",
		Service:    ev.Service,
		RootCause:  root,
		Evidence:   []string{evidence.Line("compose_error", ev.Message), evidence.KV("image", image)},
		Confidence: confidence.High,
		SuggestedFixes: []string{
			"Correct the image name/tag, or push the missing tag to the registry",
		},
	}}
}

type imagePlatformMismatchRule struct{}

func (imagePlatformMismatchRule) ID() string { return "image.platform_mismatch" }
func (imagePlatformMismatchRule) Description() string {
	return "Image platform/architecture does not match the host"
}

func (imagePlatformMismatchRule) Evaluate(ctx *engine.RunContext) []model.Finding {
	evs := eventsMatching(ctx, func(ev model.Event) bool {
		if dataString(ev, "progress_kind") == "arch_mismatch" {
			return true
		}
		return containsAny(ev.Message,
			"no matching manifest",
			"image operating system",
			"exec format error",
			"platform",
		) && containsAny(ev.Message, "manifest", "arm64", "amd64", "operating system", "platform")
	})
	if len(evs) == 0 {
		return nil
	}
	ev := evs[0]
	image := dataString(ev, "image")
	root := "image platform does not match the host"
	if image != "" {
		root = "image platform mismatch for " + image
	}
	return []model.Finding{{
		RuleID:     "image.platform_mismatch",
		Service:    ev.Service,
		RootCause:  root,
		Evidence:   []string{evidence.Line("compose_error", ev.Message), evidence.KV("image", image)},
		Confidence: confidence.High,
		SuggestedFixes: []string{
			"Pull/build for the host platform, or set `platform:` explicitly to a supported value",
		},
	}}
}
