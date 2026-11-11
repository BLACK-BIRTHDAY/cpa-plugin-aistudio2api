# In-Stream Tool Call Recovery, Abnormal Finish Surfacing, and Assistant Applet Self-Repair

Status: accepted

Because upstream Google AI Studio `GenerateContent` and `CodeAssistantOffline` occasionally fail on large tool arguments or emit malformed `function_calls` blocks after initial introductory text has already been committed to the client stream (`committed == true`), previous plugin revisions silently treated these broken turns as normal completions (`stop` in OpenAI, `end_turn` in Anthropic, `STOP` in Gemini), causing client agents (such as Cherry Studio) to freeze mid-workflow without surfacing errors.

We decided to:
1. **In-Stream Continuation Recovery (`GenerateContent`)**: When upstream terminates with `malformed_function_call` (`finishCode = 10`), `unexpected_tool_call`, or `malformed_response` after preamble text has already been emitted, verify that only plain text was committed (`committedTextOnly`), and continue the stream within the same request lifecycle using a `<partial_reply>` continuation prompt.
2. **Multi-Target Continuation Prefix Stripping**: Install `continuationPrefixStripper` to strip echoed preamble text across retry rounds using exact, whitespace-trimmed, and 20k-rune tail targets while preserving thought signatures, handling pre-text signatures, and maintaining active state across interleaved non-text events (`EventUsage`, `EventReasoning`, `EventGrounding`, `EventCitation`).
3. **Explicit Abnormal Finish Error Surfacing**: Introduce `*AbnormalFinishError` so exhausted retries on malformed tool calls or responses emit `EventError` rather than masquerading as clean `EventFinish` closures. Gate pre-error `EventUsage` and uncommitted source emission strictly on `if sentOutput`, preventing pre-header HTTP 502 errors from prematurely committing HTTP 200 SSE headers.
4. **Assistant Applet Multi-Round Self-Repair**:
   - Repair trailing commas outside JSON string literals in `repairJSONStrings` before parsing `function_calls` blocks.
   - Intercept forbidden remote workspace tools (`invalidTool`) and missing `function_calls` blocks (`malformed`) in `AssistantStreamDecoder`, pausing downstream processing of bogus remote workspace output.
   - Execute in-applet follow-up prompts (`assistantMalformedCallPrompt`, `assistantInvalidToolPrompt`) with tail prefix deduplication in `sendAssistant`, and gate outer `Generate()` retries on `!entry.model.Capabilities["assistant_route"] || err != nil` to eliminate 4x4 retry amplification.
