package main

import (
	"calculator/business"
	"calculator/presentation"
	"calculator/storage"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

// WebRTC структуры и переменные
var jwtKey = []byte("verysecretkey")

type User struct {
	Name string
	Conn *websocket.Conn
}

type Session struct {
	ID        string `json:"sessionId"`
	Caller    string `json:"caller"`
	Target    string `json:"target"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

type Server struct {
	users    map[string]*User
	sessions map[string]*Session
	mu       sync.Mutex
	upgrader websocket.Upgrader
}

func NewServer() *Server {
	return &Server{
		users:    make(map[string]*User),
		sessions: make(map[string]*Session),
		upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	}
}

func main() {
	// Инициализация компонентов калькулятора
	historyRepo := storage.NewHistoryRepository()
	interpreter := business.NewInterpreter(historyRepo)
	webHandler := presentation.NewWebHandler(interpreter)

	// Восстановление истории
	err := historyRepo.Restore()
	if err != nil {
		fmt.Printf("Ошибка восстановления истории: %v\n", err)
	}

	s := NewServer()

	http.Handle("/", http.FileServer(http.Dir("./web")))
	http.Handle("/webrtc/", http.StripPrefix("/webrtc/", http.FileServer(http.Dir("./signaling/static"))))

	http.HandleFunc("/api/calculate", webHandler.CalculateHandler)
	http.HandleFunc("/api/history", webHandler.HistoryHandler)

	http.HandleFunc("/api/calculate", webHandler.CalculateHandler)
	http.HandleFunc("/api/history", webHandler.HistoryHandler)
	http.HandleFunc("/api/auth/login", s.loginHandler)
	http.HandleFunc("/api/session", s.sessionHandler)
	http.HandleFunc("/api/session/accept", s.acceptHandler)
	http.HandleFunc("/api/session/decline", s.declineHandler)
	http.HandleFunc("/api/session/cancel", s.cancelHandler)
	http.HandleFunc("/ws", s.wsHandler)
	//http.HandleFunc("/api/call-data/", callDataHandler)

	http.HandleFunc("/api/call-data/", func(w http.ResponseWriter, r *http.Request) {
		dataId := strings.TrimPrefix(r.URL.Path, "/api/call-data/")
		log.Printf("=== ЗАПРОС ДАННЫХ === dataId: %s", dataId)

		if dataId == "" {
			log.Printf("ОШИБКА: dataId пустой")
			http.Error(w, "dataId required", http.StatusBadRequest)
			return
		}

		tempDir := os.TempDir()
		dataPath := filepath.Join(tempDir, dataId+".json")
		log.Printf("Путь к файлу: %s", dataPath)

		if _, err := os.Stat(dataPath); os.IsNotExist(err) {
			log.Printf("ОШИБКА: файл не существует: %s", dataPath)
			http.Error(w, "data not found", http.StatusNotFound)
			return
		}

		data, err := os.ReadFile(dataPath)
		if err != nil {
			log.Printf("ОШИБКА чтения файла: %v", err)
			http.Error(w, "data not found", http.StatusNotFound)
			return
		}

		log.Printf("ДАННЫЕ НАЙДЕНЫ: %s", string(data))

		if err := os.Remove(dataPath); err != nil {
			log.Printf("Предупреждение: не удалось удалить файл: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		log.Printf("=== ДАННЫЕ ОТПРАВЛЕНЫ ===")
	})

	log.Println("🚀 Сервер запущен на :8080")
	log.Println("📊 Калькулятор: http://localhost:8080")
	log.Println("📞 WebRTC звонки: http://localhost:8080/webrtc/")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func (s *Server) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": req.Username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenStr, _ := token.SignedString(jwtKey)
	json.NewEncoder(w).Encode(map[string]string{"token": tokenStr})
}

func (s *Server) wsHandler(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil || !token.Valid {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		http.Error(w, "invalid token claims", http.StatusUnauthorized)
		return
	}

	username, ok := claims["username"].(string)
	if !ok || username == "" {
		http.Error(w, "invalid username in token", http.StatusUnauthorized)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	s.mu.Lock()
	s.users[username] = &User{Name: username, Conn: conn}
	s.mu.Unlock()
	log.Println("WS connected:", username)

	s.handleWebSocketConnection(username, conn)
}

func (s *Server) handleWebSocketConnection(username string, conn *websocket.Conn) {
	defer func() {
		s.mu.Lock()
		delete(s.users, username)
		s.mu.Unlock()
		conn.Close()
		log.Println("WS disconnected:", username)
	}()

	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error for %s: %v", username, err)
			}
			break
		}

		if typ, ok := msg["type"].(string); ok && typ == "signal" {
			if data, ok := msg["data"].(map[string]interface{}); ok {
				if target, ok := data["target"].(string); ok {
					if _, hasFrom := data["from"]; !hasFrom {
						data["from"] = username
						msg["data"] = data
					}
					s.forwardSignal(username, target, msg)
				}
			}
		}
	}
}

func (s *Server) forwardSignal(from, to string, msg map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.users[to]; ok && u.Conn != nil {
		u.Conn.WriteJSON(msg)
	}
}

func (s *Server) sessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		TargetUsername string `json:"targetUsername"`
		Type           string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	caller := getUsernameFromHeader(r)
	if caller == "" {
		http.Error(w, "unauth", http.StatusUnauthorized)
		return
	}
	id := fmt.Sprintf("%s_%s_%d", caller, req.TargetUsername, time.Now().Unix())
	sess := &Session{ID: id, Caller: caller, Target: req.TargetUsername, Type: req.Type, Status: "pending", CreatedAt: time.Now().Format(time.RFC3339)}
	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()
	s.notify(req.TargetUsername, map[string]interface{}{"type": "session_updated", "data": sess})
	json.NewEncoder(w).Encode(sess)
}

func (s *Server) acceptHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionId string `json:"sessionId"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	s.updateSessionStatus(req.SessionId, "active")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) declineHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionId string `json:"sessionId"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	s.updateSessionStatus(req.SessionId, "declined")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) cancelHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		SessionId string `json:"sessionId"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	s.updateSessionStatus(req.SessionId, "cancelled")
	w.WriteHeader(http.StatusOK)
}

func (s *Server) updateSessionStatus(sessionId, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[sessionId]; ok {
		sess.Status = status
		s.notify(sess.Caller, map[string]interface{}{"type": "session_updated", "data": sess})
		s.notify(sess.Target, map[string]interface{}{"type": "session_updated", "data": sess})
	}
}

func (s *Server) notify(username string, msg interface{}) {
	if u, ok := s.users[username]; ok && u.Conn != nil {
		u.Conn.WriteJSON(msg)
	}
}

func getUsernameFromHeader(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	var tokenStr string
	fmt.Sscanf(auth, "Bearer %s", &tokenStr)
	claims := jwt.MapClaims{}
	if tokenStr == "" {
		return ""
	}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	if err != nil {
		return ""
	}
	if uname, ok := claims["username"].(string); ok {
		return uname
	}
	return ""
}
