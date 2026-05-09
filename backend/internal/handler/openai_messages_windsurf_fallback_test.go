package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldRetryOpenAIMessagesWithNativeWindsurf(t *testing.T) {
	require.True(t, shouldRetryOpenAIMessagesWithNativeWindsurf("claude-opus-4-7", "gpt-5.4"))
	require.False(t, shouldRetryOpenAIMessagesWithNativeWindsurf("claude-opus-4-7", "claude-opus-4-7"))
	require.False(t, shouldRetryOpenAIMessagesWithNativeWindsurf("unknown-model", "gpt-5.4"))
	require.False(t, shouldRetryOpenAIMessagesWithNativeWindsurf("", "gpt-5.4"))
}
