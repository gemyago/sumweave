package agentapi

import rt "github.com/gemyago/sumweave/runtime/internal"

// Matches [agent.defaultRunnerAppName] — listing uses the same app scope as runs.
const listSessionsAppName = "sumweave-runtime"

func mapListedSessionMetadata(m rt.SessionMetadata) SessionMetadata {
	title := m.Title
	if title == "" {
		title = "Session " + m.CreatedAt.Format("Jan 2 15:04")
	}
	return SessionMetadata{
		SessionId: m.SessionID,
		Title:     title,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
