package websocket

import (
	"encoding/json"
	"net/http"
	"time"
)

// SessionInfo содержит краткую информацию о сессии для мониторинга
type SessionInfo struct {
	SessionID string    `json:"sessionId"`
	UserID    uint64    `json:"userId"`
	Started   bool      `json:"started"`
	StartTime time.Time `json:"startTime"`
	LastTime  time.Time `json:"lastTime"`
}

// handleMonitorData возвращает JSON с информацией о текущих сессиях (Basic Auth)
func (s *Server) handleMonitorData(w http.ResponseWriter, r *http.Request) {
	// Basic Auth: логин admin, пароль yourpassword
	user, pass, ok := r.BasicAuth()
	if !ok || user != "admin" || pass != "yourpassword" {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	s.sessionMutex.RLock()
	defer s.sessionMutex.RUnlock()

	// Гарантируем, что sessions всегда инициализирован как пустой слайс
	sessions := make([]SessionInfo, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, SessionInfo{
			SessionID: session.ID,
			UserID:    session.UserID,
			Started:   session.Started,
			StartTime: session.StartTime,
			LastTime:  session.LastTime,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// handleMonitorPage отдаёт HTML-страницу мониторинга сессий (Basic Auth)
func (s *Server) handleMonitorPage(w http.ResponseWriter, r *http.Request) {
	user, pass, ok := r.BasicAuth()
	if !ok || user != "admin" || pass != "yourpassword" {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>WebSocket Monitor</title>
  <style>
    body { font-family: sans-serif; margin: 2em; }
    table { border-collapse: collapse; min-width: 600px; }
    th, td { border: 1px solid #ccc; padding: 6px 12px; }
    th { background: #f0f0f0; }
    caption { font-size: 1.2em; margin-bottom: 0.5em; }
  </style>
</head>
<body>
  <h1>WebSocket Sessions Monitor</h1>
  <table id="sessions">
    <caption>Active Sessions</caption>
    <thead>
      <tr>
        <th>SessionID</th>
        <th>UserID</th>
        <th>Started</th>
        <th>StartTime</th>
        <th>LastTime</th>
      </tr>
    </thead>
    <tbody></tbody>
  </table>
  <script>
    async function fetchSessions() {
      try {
        const resp = await fetch('/ws-monitor/data');
        if (!resp.ok) return;
        const data = await resp.json();
        const tbody = document.querySelector('#sessions tbody');
        tbody.innerHTML = '';
        if (data.length === 0) {
          const row = document.createElement('tr');
          const cell = document.createElement('td');
          cell.colSpan = 5;
          cell.textContent = 'No active sessions';
          row.appendChild(cell);
          tbody.appendChild(row);
        } else {
          data.forEach(s => {
            const row = document.createElement('tr');
            row.innerHTML = '<td>' + s.sessionId + '</td><td>' + s.userId + '</td><td>' + (s.started ? 'Yes' : 'No') + '</td><td>' + new Date(s.startTime).toLocaleString() + '</td><td>' + new Date(s.lastTime).toLocaleString() + '</td>';
            tbody.appendChild(row);
          });
        }
      } catch (e) {
        // ignore
      }
    }
    fetchSessions();
    setInterval(fetchSessions, 2000);
  </script>
</body>
</html>
`))
}
