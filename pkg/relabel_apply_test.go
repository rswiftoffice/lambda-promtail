package main

import (
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

// TestApplyRelabelConfigs_DeterministicOutput verifies applyRelabelConfigs
// produces identical output across many invocations for identical input.
//
// Regression test for the upstream bug where ScratchBuilder.Labels() returns
// labels in Go map iteration order (random) and relabel.Process, which needs
// sorted input, therefore produces non-deterministic results. The fix calls
// builder.Sort() before relabel.Process. If a future upstream rebase drops
// that Sort() call, this test fails.
func TestApplyRelabelConfigs_DeterministicOutput(t *testing.T) {
	defer func() { relabelConfigs = nil }()

	// Mirrors the platform default config used for AWS infra log forwarding
	// (tf-common-modules/logs-forwarder relabel-configs.json).
	configs, err := parseRelabelConfigs(`[
		{"source_labels":["__aws_log_type"],"target_label":"log_type","action":"replace"},
		{"source_labels":["__aws_cloudwatch_log_group"],"target_label":"log_group","action":"replace"},
		{"source_labels":["__aws_s3_log_lb"],"target_label":"loadbalancer","action":"replace"}
	]`)
	require.NoError(t, err)
	relabelConfigs = configs

	input := model.LabelSet{
		"__aws_log_type":             "cloudwatch",
		"__aws_cloudwatch_log_group": "/aws/lambda/test",
		"__aws_cloudwatch_owner":     "123456789012",
		"aws_account_id":             "123456789012",
		"project_name":               "test-app",
		"project_env":                "dev",
	}

	const iters = 1000
	var firstOut model.LabelSet
	for i := 0; i < iters; i++ {
		in := make(model.LabelSet, len(input))
		for k, v := range input {
			in[k] = v
		}
		out := applyRelabelConfigs(in)

		if i == 0 {
			firstOut = out
			require.Equal(t, model.LabelValue("cloudwatch"), out["log_type"], "log_type rename failed")
			require.Equal(t, model.LabelValue("/aws/lambda/test"), out["log_group"], "log_group rename failed")
			continue
		}
		require.Equal(t, firstOut, out, "non-deterministic output at iteration %d", i)
	}
}

// TestApplyRelabelConfigs_NoConfigsPassThrough verifies an empty config
// returns the input untouched.
func TestApplyRelabelConfigs_NoConfigsPassThrough(t *testing.T) {
	defer func() { relabelConfigs = nil }()
	relabelConfigs = nil

	in := model.LabelSet{"a": "1", "b": "2"}
	out := applyRelabelConfigs(in)
	require.Equal(t, in, out)
}
