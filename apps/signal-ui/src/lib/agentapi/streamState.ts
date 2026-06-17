import type {
  AgentStreamEvent,
  SessionBoundEvent,
  SessionStatusEvent,
  StreamErrorEvent,
  StreamEvent,
} from './types'

export interface ToolCallEntry {
  id: string
  name: string
  args: Record<string, unknown>
  response?: Record<string, unknown>
}

export interface AssistantTextPart {
  kind: 'text'
  id: string
  text: string
}

export interface AssistantToolCallPart extends ToolCallEntry {
  kind: 'toolCall'
}

export type AssistantTurnPart = AssistantTextPart | AssistantToolCallPart

/**
 * UI-facing state for a single in-flight agent run (one POST → SSE until done/error).
 * `applyAgentStreamEvent` only interprets stream events; the caller sets `busy` when
 * starting a run and may reset fields when starting a new turn.
 */
export interface AgentRunStreamState {
  sessionId: string | null
  /** False after `done` or `error`; true while chunks may still arrive. */
  busy: boolean
  /**
   * Assistant text for the current turn. Each model `agent` event appends text from
   * `content.parts[].text`. Events with `content.role === 'user'` are ignored (ADK emits user and
   * model turns as `agent` SSE events).
   */
  assistantTurnText: string
  error: string | null
  /** Set by `sessionStatus` event: true when a run is active, false when idle. */
  sessionActive: boolean | null
  /** Assistant turn content in the order it arrived from the stream. */
  assistantTurnParts: AssistantTurnPart[]
  /** Tool calls accumulated in the current assistant turn. */
  toolCalls: ToolCallEntry[]
}

export function createInitialAgentRunStreamState(
  overrides?: Partial<AgentRunStreamState>,
): AgentRunStreamState {
  return {
    sessionId: null,
    busy: true,
    assistantTurnText: '',
    error: null,
    sessionActive: null,
    assistantTurnParts: [],
    toolCalls: [],
    ...overrides,
  }
}

function applySessionBound(state: AgentRunStreamState, event: SessionBoundEvent): AgentRunStreamState {
  return { ...state, sessionId: event.sessionId }
}

function applySessionStatus(state: AgentRunStreamState, event: SessionStatusEvent): AgentRunStreamState {
  return { ...state, sessionActive: event.status === 'active' }
}

function nextTextPartID(parts: AssistantTurnPart[]): string {
  const textPartCount = parts.filter((part) => part.kind === 'text').length
  return `text-${textPartCount + 1}`
}

function mergeTextDelta(existing: string, delta: string, isFinalSnapshot: boolean): string {
  if (!isFinalSnapshot || existing.length === 0) {
    return existing + delta
  }
  if (delta === existing) {
    return existing
  }
  if (delta.startsWith(existing)) {
    return delta
  }
  return existing + delta
}

function snapshotAssistantText(parts: AssistantTurnPart[]): string {
  return parts
    .filter((part): part is AssistantTextPart => part.kind === 'text')
    .map((part) => part.text)
    .join('')
}

function snapshotToolCalls(parts: AssistantTurnPart[]): ToolCallEntry[] {
  return parts
    .filter((part): part is AssistantToolCallPart => part.kind === 'toolCall')
    .map((part) => ({
      id: part.id,
      name: part.name,
      args: part.args,
      ...(part.response !== undefined ? { response: part.response } : {}),
    }))
}

function applyAgent(state: AgentRunStreamState, event: AgentStreamEvent): AgentRunStreamState {
  const eventParts = event.content?.parts
  if (!eventParts?.length) {
    return state
  }

  const ignoreText = event.content?.role === 'user'
  let nextParts = state.assistantTurnParts

  for (const part of eventParts) {
    if (part.toolCall) {
      const { id, name, args } = part.toolCall
      nextParts = [
        ...nextParts,
        {
          kind: 'toolCall',
          id,
          name,
          args: (args as Record<string, unknown> | undefined) ?? {},
        },
      ]
      continue
    }

    if (part.toolResult) {
      const { id, response } = part.toolResult
      nextParts = nextParts.map((entry) =>
        entry.kind === 'toolCall' && entry.id === id
          ? { ...entry, response: (response as Record<string, unknown> | undefined) ?? {} }
          : entry,
      )
      continue
    }

    if (part.text === undefined || ignoreText) {
      continue
    }

    const lastPart = nextParts[nextParts.length - 1]
    if (part.text.trim().length === 0 && lastPart?.kind !== 'text') {
      continue
    }
    if (lastPart?.kind === 'text') {
      nextParts = [
        ...nextParts.slice(0, -1),
        {
          ...lastPart,
          text: mergeTextDelta(lastPart.text, part.text, event.partial === false),
        },
      ]
      continue
    }

    nextParts = [
      ...nextParts,
      {
        kind: 'text',
        id: nextTextPartID(nextParts),
        text: part.text,
      },
    ]
  }

  if (nextParts === state.assistantTurnParts) {
    return state
  }

  return {
    ...state,
    assistantTurnParts: nextParts,
    assistantTurnText: snapshotAssistantText(nextParts),
    toolCalls: snapshotToolCalls(nextParts),
  }
}

function applyError(state: AgentRunStreamState, event: StreamErrorEvent): AgentRunStreamState {
  return { ...state, busy: false, error: event.message }
}

function applyDone(state: AgentRunStreamState): AgentRunStreamState {
  return { ...state, busy: false }
}

/** One row in the chat transcript (UI uses `assistant`; SSE uses `model`). */
export type ChatTranscriptMessage = {
  role: 'user' | 'assistant'
  text: string
  parts?: AssistantTurnPart[]
  toolCalls?: ToolCallEntry[]
}

export function assistantMessageFromStreamState(
  state: AgentRunStreamState,
): ChatTranscriptMessage | null {
  const text = state.assistantTurnText
  const toolCalls = state.toolCalls
  const parts = state.assistantTurnParts
  if (!text && toolCalls.length === 0 && parts.length === 0) {
    return null
  }
  const row: ChatTranscriptMessage = { role: 'assistant', text }
  if (parts.length > 0) {
    row.parts = [...parts]
  }
  if (toolCalls.length > 0) {
    row.toolCalls = [...toolCalls]
  }
  return row
}

/**
 * Folds one historical `agent` event from an idle `readSession` stream into scratch assistant state
 * and optional transcript rows. Uses the same {@link applyAgent} accumulation rules as live runs;
 * flushes on `turnComplete` or before a user event (role transition).
 */
function toolCallsStillPending(row: ChatTranscriptMessage): boolean {
  return row.toolCalls?.some((tc) => tc.response === undefined) ?? false
}

export function applyIdleHistoryAgentEvent(
  state: AgentRunStreamState,
  event: AgentStreamEvent,
): { state: AgentRunStreamState; appended: ChatTranscriptMessage[] } {
  const appended: ChatTranscriptMessage[] = []

  // ADK emits tool results as `agent` events with role=user; merge them before flushing transcript.
  if (event.content?.role === 'user') {
    let next = applyAgentStreamEvent(state, event)
    const userText = event.content.parts?.map((p) => p.text).join('') ?? ''
    const assistantRow = assistantMessageFromStreamState(next)
    if (assistantRow && !toolCallsStillPending(assistantRow)) {
      appended.push(assistantRow)
      next = { ...next, assistantTurnText: '', assistantTurnParts: [], toolCalls: [] }
    }
    if (userText) {
      appended.push({ role: 'user', text: userText })
    }
    return { state: next, appended }
  }

  let next = applyAgentStreamEvent(state, event)
  if (event.turnComplete) {
    const row = assistantMessageFromStreamState(next)
    if (row && !toolCallsStillPending(row)) {
      appended.push(row)
      next = { ...next, assistantTurnText: '', assistantTurnParts: [], toolCalls: [] }
    }
  }
  return { state: next, appended }
}

/**
 * Pure reducer for stream events. `agent` events append text; other kinds update fields as implemented.
 */
export function applyAgentStreamEvent(
  state: AgentRunStreamState,
  event: StreamEvent,
): AgentRunStreamState {
  switch (event.event) {
    case 'sessionBound':
      return applySessionBound(state, event)
    case 'sessionStatus':
      return applySessionStatus(state, event)
    case 'agent':
      return applyAgent(state, event)
    case 'error':
      return applyError(state, event)
    case 'done':
      return applyDone(state)
  }
}
