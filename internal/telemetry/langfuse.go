// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package telemetry

import (
	"encoding/json"

	"go.opentelemetry.io/otel/trace"

	"github.com/alibaba/open-code-review/internal/llm"
)

// applyTraceAttrs stamps the configured OCR_TRACE_* attributes on every span.
// Langfuse accepts trace-level attributes (name, session, release,
// environment, tags) from any span in the trace, so applying them broadly is
// the simplest way to keep them consistent.
func applyTraceAttrs(span trace.Span) {
	cfg := ResolveConfig("")
	if cfg.TraceName != "" {
		SetAttr(span, "langfuse.trace.name", cfg.TraceName)
	}
	if cfg.TraceEnv != "" {
		SetAttr(span, "langfuse.environment", cfg.TraceEnv)
	}
	if cfg.TraceSession != "" {
		SetAttr(span, "langfuse.session.id", cfg.TraceSession)
	}
	if cfg.TraceRelease != "" {
		SetAttr(span, "langfuse.release", cfg.TraceRelease)
	}
	if cfg.TraceTags != "" {
		SetAttr(span, "langfuse.tags", cfg.TraceTags)
	}
}

// SpanContentEnabled reports whether span-level content capture is on
// (OCR_TRACE_CONTENT=1). Content attributes carry prompts, completions and
// tool payloads — off by default since they can be large and sensitive.
func SpanContentEnabled() bool {
	if !IsEnabled() {
		return false
	}
	return ResolveConfig("").SpanContent
}

func setJSONAttr(span trace.Span, key string, v any) {
	if v == nil {
		return
	}
	b, err := json.Marshal(v)
	if err == nil {
		SetAttr(span, key, string(b))
	}
}

// RecordLLMInput marks an llm.request span as a Langfuse generation and
// attaches the outgoing messages as its input. sessionID is used as
// langfuse.session.id only when OCR_TRACE_SESSION does not pin one.
func RecordLLMInput(span trace.Span, sessionID, model string, input any) {
	if span == nil || !SpanContentEnabled() {
		return
	}
	SetAttr(span, "langfuse.observation.type", "generation")
	SetAttr(span, "langfuse.observation.model.name", model)
	SetAttr(span, "gen_ai.request.model", model)
	if sessionID != "" && ResolveConfig("").TraceSession == "" {
		SetAttr(span, "langfuse.session.id", sessionID)
	}
	setJSONAttr(span, "langfuse.observation.input", input)
}

// RecordLLMOutput attaches the completion — content, tool calls, reasoning
// and usage — to an llm.request span after the response arrives.
func RecordLLMOutput(span trace.Span, content string, toolCalls any, reasoning string, usage *llm.UsageInfo) {
	if span == nil || !SpanContentEnabled() {
		return
	}
	out := map[string]any{"content": content}
	if toolCalls != nil {
		out["tool_calls"] = toolCalls
	}
	setJSONAttr(span, "langfuse.observation.output", out)
	if reasoning != "" {
		SetAttr(span, "langfuse.observation.metadata.reasoning_content", reasoning)
	}
	if usage != nil {
		SetAttr(span, "gen_ai.usage.input_tokens", usage.PromptTokens)
		SetAttr(span, "gen_ai.usage.output_tokens", usage.CompletionTokens)
		SetAttr(span, "gen_ai.usage.total_tokens", usage.TotalTokens)
		setJSONAttr(span, "langfuse.observation.usage_details", map[string]int64{
			"input":                usage.PromptTokens,
			"output":               usage.CompletionTokens,
			"total":                usage.TotalTokens,
			"cache_read_input":     usage.CacheReadTokens,
			"cache_creation_input": usage.CacheWriteTokens,
		})
	}
}

// RecordToolInput marks a tool span as a Langfuse tool observation and
// attaches its arguments. Call right after StartToolSpan so the input is
// recorded even when execution fails.
func RecordToolInput(span trace.Span, input any) {
	if span == nil || !SpanContentEnabled() {
		return
	}
	SetAttr(span, "langfuse.observation.type", "tool")
	setJSONAttr(span, "langfuse.observation.input", input)
}

// RecordToolOutput attaches a tool result (or error string) to a tool span.
func RecordToolOutput(span trace.Span, output any) {
	if span == nil || !SpanContentEnabled() {
		return
	}
	setJSONAttr(span, "langfuse.observation.output", output)
}
