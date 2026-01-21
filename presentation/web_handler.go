package presentation

import (
	"calculator/business"
	"encoding/json"
	"fmt"
	"net/http"
)

type WebHandler struct {
	interpreter *business.Interpreter
}

func NewWebHandler(interpreter *business.Interpreter) *WebHandler {
	return &WebHandler{interpreter: interpreter}
}

func (h *WebHandler) CalculateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Command string `json:"command"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	result, err := h.interpreter.Execute(req.Command)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Ошибка: " + err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("%v", result),
	})
}

func (h *WebHandler) HistoryHandler(w http.ResponseWriter, r *http.Request) {
	history := h.interpreter.GetHistory()
	json.NewEncoder(w).Encode(history)
}
