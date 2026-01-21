package business

import (
	"bytes"
	"calculator/storage"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type RequestClassification struct {
	Type        string `json:"type"`
	URL         string `json:"url,omitempty"`
	Action      string `json:"action,omitempty"`
	Description string `json:"description,omitempty"`
}

type ContentSummaryRequest struct {
	Content string `json:"content"`
	Task    string `json:"task"`
}

type Interpreter struct {
	variables      map[string]interface{}
	historyRepo    *storage.HistoryRepository
	httpClient     *http.Client
	customSafeDirs []string
	callUsername   string 
	callToken      string 
}

func NewInterpreter(historyRepo *storage.HistoryRepository) *Interpreter {
	return &Interpreter{
		variables:      make(map[string]interface{}),
		historyRepo:    historyRepo,
		httpClient:     &http.Client{Timeout: 60 * time.Second},
		customSafeDirs: []string{},
		callUsername:   "",
		callToken:      "",
	}
}

func (i *Interpreter) handleCallLogin(input string) (interface{}, error) {
	parts := strings.SplitN(input, " ", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("неверный формат команды. Используйте: войти как [имя]")
	}

	username := parts[2]
	token, err := i.loginToCallServer(username)
	if err != nil {
		return nil, err
	}

	i.callUsername = username
	i.callToken = token

	return fmt.Sprintf("Успешно вошли как %s. Теперь можете звонить!", username), nil
}
func (i *Interpreter) handleCallCommand(input string) (interface{}, error) {
	if i.callToken == "" {
		return nil, fmt.Errorf("сначала выполните вход с помощью команды 'войти как [имя]'")
	}

	parts := strings.SplitN(input, " ", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("неверный формат команды. Используйте: позвонить [имя]")
	}

	target := parts[1]

	fmt.Printf("=== DEBUG handleCallCommand ===\n")
	fmt.Printf("i.callUsername: '%s'\n", i.callUsername)
	fmt.Printf("target: '%s'\n", target)
	callerDataID := fmt.Sprintf("caller_%d", time.Now().UnixNano())
	targetDataID := fmt.Sprintf("target_%d", time.Now().UnixNano())
	tempDir := os.TempDir()
	fmt.Printf("Временная директория: %s\n", tempDir)
	callerData := map[string]string{
		"token":    i.callToken,
		"username": i.callUsername,
		"target":   target,
		"autoCall": "true",
	}

	callerDataFile := filepath.Join(tempDir, callerDataID+".json")
	callerDataJSON, _ := json.Marshal(callerData)

	fmt.Printf("Создаю файл: %s\n", callerDataFile)
	fmt.Printf("Данные caller: %s\n", string(callerDataJSON))

	if err := os.WriteFile(callerDataFile, callerDataJSON, 0644); err != nil {
		fmt.Printf("ОШИБКА записи файла caller: %v\n", err)
		return nil, fmt.Errorf("ошибка сохранения данных: %v", err)
	}

	if _, err := os.Stat(callerDataFile); err == nil {
		fmt.Printf("Файл caller успешно создан\n")
	} else {
		fmt.Printf("ОШИБКА: файл caller не создан: %v\n", err)
	}

	url1 := fmt.Sprintf("http://localhost:8080/?dataId=%s", callerDataID)
	fmt.Printf("URL 1: %s\n", url1)
	_, _, err := i.openBrowser(url1)
	if err != nil {
		return nil, fmt.Errorf("ошибка при открытии браузера: %v", err)
	}

	targetToken, err := i.loginToCallServer(target)
	if err != nil {
		return nil, fmt.Errorf("ошибка входа для %s: %v", target, err)
	}

	targetData := map[string]string{
		"token":    targetToken,
		"username": target,
	}

	targetDataFile := filepath.Join(tempDir, targetDataID+".json")
	targetDataJSON, _ := json.Marshal(targetData)
	fmt.Printf("Создаю файл: %s\n", targetDataFile)
	fmt.Printf("Данные target: %s\n", string(targetDataJSON))
	if err := os.WriteFile(targetDataFile, targetDataJSON, 0644); err != nil {
		fmt.Printf("ОШИБКА записи файла target: %v\n", err)
		return nil, fmt.Errorf("ошибка сохранения данных: %v", err)
	}
	if _, err := os.Stat(targetDataFile); err == nil {
		fmt.Printf("Файл target успешно создан\n")
	} else {
		fmt.Printf("ОШИБКА: файл target не создан: %v\n", err)
	}

	time.Sleep(2 * time.Second)
	url2 := fmt.Sprintf("http://localhost:8080/?dataId=%s", targetDataID)
	fmt.Printf("URL 2: %s\n", url2)
	fmt.Printf("=== END DEBUG ===\n")
	_, _, err = i.openBrowser(url2)
	if err != nil {
		return nil, fmt.Errorf("ошибка при открытии второго браузера: %v", err)
	}
	return fmt.Sprintf("Открываю звонок: %s звонит %s", i.callUsername, target), nil
}

func (i *Interpreter) loginToCallServer(username string) (string, error) {
	data := map[string]string{"username": username}
	jsonData, _ := json.Marshal(data)
	resp, err := http.Post("http://localhost:8080/api/auth/login", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("ошибка подключения к серверу звонков: %v", err)
	}
	defer resp.Body.Close()
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ошибка входа: %s", result)
	}
	return result["token"], nil
}

func (i *Interpreter) Execute(input string) (interface{}, error) {
	i.historyRepo.AddCommand(input)
	input = strings.TrimSpace(input)
	if input == "history" || input == "история" {
		return i.GetHistory(), nil
	}
	if strings.HasPrefix(input, "войти как ") {
		return i.handleCallLogin(input)
	}

	if strings.HasPrefix(input, "позвонить ") {
		return i.handleCallCommand(input)
	}
	if strings.HasPrefix(strings.ToLower(input), "curl ") {
		return i.handleCurlCommand(input)
	}

	if strings.Contains(input, "=") && !strings.Contains(input, "==") {
		parts := strings.SplitN(input, "=", 2)
		if len(parts) == 2 {
			variable := strings.TrimSpace(parts[0])
			expression := strings.TrimSpace(parts[1])

			if isValidVariableName(variable) {
				if strings.HasPrefix(strings.ToLower(expression), "curl ") {
					return i.handleCurlAssignment(variable, expression)
				}
			}
		}
	}

	if strings.Contains(input, "http://") || strings.Contains(input, "https://") || strings.Contains(input, "www.") {
		if result, handled, err := i.handleApplicationOpening(input); handled {
			return result, err
		}
		if result, err := i.handleComplexRequest(input); err == nil {
			return result, nil
		}
	}

	if i.isCalculableExpression(input) {
		result, err := i.evaluateExpression(input)
		if err == nil {
			return result, nil
		}
		return nil, err
	}

	if result, handled, err := i.handleApplicationOpening(input); handled {
		return result, err
	}

	textResponse, err := i.SendTextToDeepSeek(input)
	if err != nil {
		return nil, err
	}
	return textResponse, nil
}
func (i *Interpreter) isSimpleFileOpenRequest(input string) bool {
	lowerInput := strings.ToLower(input)
	simpleCommands := []string{
		"открой файл", "open file",
		"включи видео", "play video",
		"включи музыку", "play music",
		"открой документ", "open document",
	}

	for _, cmd := range simpleCommands {
		if strings.Contains(lowerInput, cmd) && i.containsFileExtension(input) {
			return true
		}
	}
	return false
}

func (i *Interpreter) handleApplicationOpening(input string) (interface{}, bool, error) {
	lowerInput := strings.ToLower(input)
	if strings.Contains(lowerInput, "расскажи") || strings.Contains(lowerInput, "проанализируй") ||
		strings.Contains(lowerInput, "дай сводку") || strings.Contains(lowerInput, "что на") {
		return nil, false, nil
	}
	if i.containsFileExtension(input) {
		if result, handled, err := i.tryOpenFile(input); handled {
			return result, handled, err
		}
	}
	if i.isOpenMediaPlayerRequest(input) {
		return i.openMediaPlayer(input)
	}

	if i.isOpenFileRequest(input) {
		return i.openFile(input)
	}

	if i.isOpenBrowserRequest(input) {
		return i.openBrowser(input)
	}
	return nil, false, nil
}
func (i *Interpreter) containsFileExtension(input string) bool {
	fileExtensions := []string{
		".pdf", ".doc", ".docx", ".txt", ".rtf", ".odt", ".xls", ".xlsx", ".ppt", ".pptx",
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".svg",
		".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v",
		".mp3", ".wav", ".flac", ".ogg", ".m4a", ".aac",
		".zip", ".rar", ".7z", ".tar", ".gz",
	}

	inputLower := strings.ToLower(input)
	for _, ext := range fileExtensions {
		if strings.Contains(inputLower, ext) {
			return true
		}
	}
	return false
}

func (i *Interpreter) tryOpenFile(input string) (interface{}, bool, error) {
	filename := i.extractFilenameFromInput(input)
	if filename == "" {
		return nil, false, nil
	}
	filePath, err := i.findFileInSafeDirectories(filename)
	if err != nil {
		return nil, true, fmt.Errorf("файл '%s' не найден в безопасных директориях", filename)
	}
	return i.openFileWithDefaultApp(filePath)
}

func (i *Interpreter) isOpenBrowserRequest(input string) bool {
	lowerInput := strings.ToLower(input)
	analysisKeywords := []string{"расскажи", "проанализируй", "дай сводку", "анализ", "что на", "summary", "analyze"}
	for _, keyword := range analysisKeywords {
		if strings.Contains(lowerInput, keyword) {
			return false
		}
	}
	browserKeywords := []string{"открой сайт", "open website", "открой в браузере", "open in browser", "зайди на", "go to", "посети", "visit"}
	if strings.Contains(lowerInput, "http://") || strings.Contains(lowerInput, "https://") ||
		strings.Contains(lowerInput, "www.") || i.containsCommonDomain(lowerInput) {
		return true
	}

	for _, keyword := range browserKeywords {
		if strings.Contains(lowerInput, keyword) {
			return true
		}
	}
	return false
}

func (i *Interpreter) containsCommonDomain(input string) bool {
	commonDomains := []string{".com", ".ru", ".org", ".net", ".io", ".info", ".рф", ".su"}
	for _, domain := range commonDomains {
		if strings.Contains(input, domain) {
			return true
		}
	}
	return false
}

func (i *Interpreter) isOpenMediaPlayerRequest(input string) bool {
	lowerInput := strings.ToLower(input)
	mediaKeywords := []string{"видео", "video", "музыка", "music", "фильм", "movie", "vlc", "проигрыватель", "player", "mp4", "avi", "mkv", "mp3", "wav", "включи", "play"}

	for _, keyword := range mediaKeywords {
		if strings.Contains(lowerInput, keyword) {
			return true
		}
	}
	return false
}
func (i *Interpreter) isOpenFileRequest(input string) bool {
	lowerInput := strings.ToLower(input)

	if strings.Contains(lowerInput, "открой сайт") || strings.Contains(lowerInput, "open website") ||
		strings.Contains(lowerInput, "открой в браузере") || strings.Contains(lowerInput, "open in browser") {
		return false
	}

	if strings.Contains(input, "http://") || strings.Contains(input, "https://") || strings.Contains(input, "www.") {
		return false
	}

	fileKeywords := []string{"файл", "file", "открой", "open", "документ", "document", "текст", "text", "pdf", "doc", "docx", "txt"}
	if i.containsFileExtension(input) {
		return true
	}

	for _, keyword := range fileKeywords {
		if strings.Contains(lowerInput, keyword) {
			return true
		}
	}

	if strings.HasPrefix(lowerInput, "открой ") || strings.HasPrefix(lowerInput, "open ") {
		words := strings.Fields(lowerInput)
		for _, word := range words {
			if strings.Contains(word, ".") && !i.isLikelyDomain(word) {
				return true
			}
		}
	}

	return false
}

func (i *Interpreter) openBrowser(input string) (interface{}, bool, error) {

	url := i.extractURLFromInput(input)
	if url == "" {

		classification, err := i.classifyRequest(input)
		if err == nil && classification.URL != "" {
			url = classification.URL
		} else {

			url = i.extractDomainFromInput(input)
		}
	}

	if url == "" {
		return "Пожалуйста, укажите URL или название сайта для открытия", true, nil
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		browsers := []string{"xdg-open", "google-chrome", "chrome", "firefox", "safari"}
		for _, browser := range browsers {
			if path, err := exec.LookPath(browser); err == nil {
				cmd = exec.Command(path, url)
				break
			}
		}
		if cmd == nil {
			cmd = exec.Command("xdg-open", url)
		}
	default:
		return nil, true, fmt.Errorf("неподдерживаемая операционная система")
	}

	if err := cmd.Start(); err != nil {
		return nil, true, fmt.Errorf("ошибка при открытии браузера: %v", err)
	}

	return fmt.Sprintf("Открываю в браузере: %s", url), true, nil
}

func (i *Interpreter) openMediaPlayer(input string) (interface{}, bool, error) {
	filename := i.extractFilenameFromInput(input)
	if filename == "" {
		return "Пожалуйста, укажите название видео или аудио файла", true, nil
	}
	filePath, err := i.findFileInSafeDirectories(filename)
	if err != nil {
		return nil, true, fmt.Errorf("файл не найден в безопасных директориях: %v", err)
	}

	return i.openMediaPlayerWithFile(filePath)
}

func (i *Interpreter) openMediaPlayerWithFile(filePath string) (interface{}, bool, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", filePath)
	case "darwin":
		if path, err := exec.LookPath("vlc"); err == nil {
			cmd = exec.Command(path, filePath)
		} else {
			cmd = exec.Command("open", filePath)
		}
	case "linux":
		players := []string{"vlc", "mpv", "smplayer", "xdg-open"}
		for _, player := range players {
			if path, err := exec.LookPath(player); err == nil {
				cmd = exec.Command(path, filePath)
				break
			}
		}
		if cmd == nil {
			cmd = exec.Command("xdg-open", filePath)
		}
	default:
		return nil, true, fmt.Errorf("неподдерживаемая операционная система")
	}

	if err := cmd.Start(); err != nil {
		return nil, true, fmt.Errorf("ошибка при открытии медиаплеера: %v", err)
	}

	return fmt.Sprintf("Открываю в медиаплеере: %s", filepath.Base(filePath)), true, nil
}

func (i *Interpreter) openFile(input string) (interface{}, bool, error) {
	filename := i.extractFilenameFromInput(input)
	if filename == "" {
		return "Пожалуйста, укажите название файла", true, nil
	}

	filePath, err := i.findFileInSafeDirectories(filename)
	if err != nil {
		return nil, true, fmt.Errorf("файл не найден в безопасных директориях: %v", err)
	}

	return i.openFileWithDefaultApp(filePath)
}

func (i *Interpreter) openFileWithDefaultApp(filePath string) (interface{}, bool, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", filePath)
	case "darwin":
		cmd = exec.Command("open", filePath)
	case "linux":
		cmd = exec.Command("xdg-open", filePath)
	default:
		return nil, true, fmt.Errorf("неподдерживаемая операционная система")
	}

	if err := cmd.Start(); err != nil {
		return nil, true, fmt.Errorf("ошибка при открытии файла: %v", err)
	}

	return fmt.Sprintf("Открываю файл: %s", filepath.Base(filePath)), true, nil
}

func (i *Interpreter) extractURLFromInput(input string) string {
	words := strings.Fields(input)
	for _, word := range words {
		cleanWord := strings.Trim(word, ",.!?;:\"'")
		if strings.HasPrefix(cleanWord, "http://") || strings.HasPrefix(cleanWord, "https://") || strings.HasPrefix(cleanWord, "www.") {
			return cleanWord
		}
	}
	return ""
}

func (i *Interpreter) extractDomainFromInput(input string) string {
	words := strings.Fields(input)
	commonDomains := []string{".com", ".ru", ".org", ".net", ".io", ".info", ".рф"}

	for _, word := range words {
		cleanWord := strings.Trim(word, ",.!?;:\"'")
		for _, domain := range commonDomains {
			if strings.Contains(cleanWord, domain) && !strings.Contains(cleanWord, " ") {
				return cleanWord
			}
		}
	}
	return ""
}

func (i *Interpreter) extractFilenameFromInput(input string) string {

	fileExtensions := []string{
		".pdf", ".doc", ".docx", ".txt", ".rtf", ".odt", ".xls", ".xlsx", ".ppt", ".pptx",
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".svg",
		".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v",
		".mp3", ".wav", ".flac", ".ogg", ".m4a", ".aac",
		".zip", ".rar", ".7z", ".tar", ".gz",
	}

	words := strings.Fields(input)
	for _, word := range words {
		cleanWord := strings.Trim(word, ",.!?;:\"'")
		for _, ext := range fileExtensions {
			if strings.HasSuffix(strings.ToLower(cleanWord), ext) {
				return cleanWord
			}
		}
	}

	for _, word := range words {
		cleanWord := strings.Trim(word, ",.!?;:\"'")
		if strings.Contains(cleanWord, ".") && !i.isLikelyDomain(cleanWord) {
			return cleanWord
		}
	}

	return ""
}

func (i *Interpreter) isLikelyDomain(word string) bool {
	if strings.HasPrefix(word, "www.") || strings.HasPrefix(word, "http") {
		return true
	}

	commonTlds := []string{".com", ".ru", ".org", ".net", ".io", ".info", ".рф"}
	for _, tld := range commonTlds {
		if strings.HasSuffix(word, tld) {
			return true
		}
	}

	return false
}

func (i *Interpreter) getSafeDirectories() []string {
	homeDir, _ := os.UserHomeDir()
	safeDirs := []string{
		filepath.Join(homeDir, "Downloads"),
		filepath.Join(homeDir, "Documents"),
		filepath.Join(homeDir, "Music"),
		filepath.Join(homeDir, "Videos"),
		filepath.Join(homeDir, "Pictures"),
		filepath.Join(homeDir, "Desktop"),
	}

	if cwd, err := os.Getwd(); err == nil {
		safeDirs = append(safeDirs, cwd)
		subdirs := []string{"documents", "downloads", "files", "docs", "рабочие", "документы"}
		for _, subdir := range subdirs {
			safeDirs = append(safeDirs, filepath.Join(cwd, subdir))
		}
	}
	if len(i.customSafeDirs) > 0 {
		safeDirs = append(safeDirs, i.customSafeDirs...)
	}

	return safeDirs
}
func (i *Interpreter) findFileInSafeDirectories(filename string) (string, error) {
	safeDirs := i.getSafeDirectories()
	for _, dir := range safeDirs {
		fullPath := filepath.Join(dir, filename)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
	}
	filenameLower := strings.ToLower(filename)
	for _, dir := range safeDirs {
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, file := range files {
			if !file.IsDir() {
				fileName := file.Name()
				if strings.ToLower(fileName) == filenameLower {
					return filepath.Join(dir, fileName), nil
				}
			}
		}
	}
	for _, dir := range safeDirs {
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, file := range files {
			if !file.IsDir() {
				fileName := file.Name()
				if strings.Contains(strings.ToLower(fileName), filenameLower) {
					return filepath.Join(dir, fileName), nil
				}
			}
		}
	}

	return "", fmt.Errorf("файл '%s' не найден в безопасных директориях", filename)
}

func (i *Interpreter) handleComplexRequest(input string) (interface{}, error) {
	classification, err := i.classifyRequest(input)
	if err != nil {
		return nil, err
	}

	switch classification.Type {
	case "open_website":

		result, err := i.handleWebsiteOpening(classification, input)
		if err != nil {
			return nil, err
		}

		if strings.Contains(strings.ToLower(input), "открой") || strings.Contains(strings.ToLower(input), "open") {
			go i.openBrowserSilent(classification.URL) 
		}

		return result, nil
	case "calculation":
		return nil, fmt.Errorf("не удалось вычислить выражение")
	case "information":
		return i.SendTextToDeepSeek(input)
	default:
		return nil, fmt.Errorf("неизвестный тип запроса")
	}
}
func (i *Interpreter) openBrowserSilent(url string) {
	if url == "" {
		return
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}

	cmd.Start()
}

func (i *Interpreter) classifyRequest(input string) (*RequestClassification, error) {
	systemPrompt := `Ты - классификатор запросов. Проанализируй запрос пользователя и определи его тип.
Доступные типы:
- "open_website" - запрос на открытие сайта и выполнение действий с ним (проанализировать, рассказать, дать сводку и т.д.)
- "calculation" - математические вычисления  
- "information" - информационный запрос (объяснение, справка и т.д.)

Особые случаи:
- Если запрос содержит "открой сайт" и "расскажи/проанализируй/дай сводку" - это "open_website"
- Если запрос содержит URL и просьбу что-то сделать с содержимым - это "open_website"
- Если просто "открой URL" без анализа - это не наш случай

Верни ответ в формате JSON с полями: type, url (если есть), action, description.`

	requestData := map[string]interface{}{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": systemPrompt,
			},
			{
				"role":    "user",
				"content": input,
			},
		},
		"stream":     false,
		"max_tokens": 500,
	}

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания JSON: %v", err)
	}

	proxyURL := "https://deproxy.kchugalinskiy.ru/deeproxy/api/completions"
	req, err := http.NewRequest("POST", proxyURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("41-2", "U0dMUjFs")

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка при выполнении запроса классификации: %v", err)
	}
	defer resp.Body.Close()

	responseBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ошибка от сервера классификации: %s", resp.Status)
	}

	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	err = json.Unmarshal(responseBytes, &apiResponse)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON ответа классификации: %v", err)
	}

	if len(apiResponse.Choices) == 0 {
		return nil, fmt.Errorf("пустой ответ от классификатора")
	}

	var classification RequestClassification
	jsonStart := strings.Index(apiResponse.Choices[0].Message.Content, "{")
	jsonEnd := strings.LastIndex(apiResponse.Choices[0].Message.Content, "}") + 1

	if jsonStart == -1 || jsonEnd == 0 {
		return nil, fmt.Errorf("не удалось найти JSON в ответе классификатора")
	}

	jsonStr := apiResponse.Choices[0].Message.Content[jsonStart:jsonEnd]
	err = json.Unmarshal([]byte(jsonStr), &classification)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга классификации: %v", err)
	}

	return &classification, nil
}

func (i *Interpreter) handleWebsiteOpening(classification *RequestClassification, originalRequest string) (string, error) {
	if classification.URL == "" {
		return "", fmt.Errorf("URL не указан в классификации")
	}

	content, err := i.handleCurlCommand("curl " + classification.URL)
	if err != nil {
		return "", fmt.Errorf("ошибка при получении содержимого сайта: %v", err)
	}

	if len(content) > 10000 {
		content = content[:10000] + "... [контент обрезан]"
	}

	return i.analyzeWebsiteContent(content, originalRequest, classification)
}

func (i *Interpreter) analyzeWebsiteContent(content, originalRequest string, classification *RequestClassification) (string, error) {
	systemPrompt := `Ты - ассистент для анализа веб-страниц. Проанализируй предоставленное содержимое сайта и выполни задачу пользователя.
Будь кратким и информативным. Если содержимое слишком большое, сосредоточься на основных моментах.`

	userPrompt := fmt.Sprintf(`Запрос пользователя: "%s"

Содержимое сайта %s:
%s

Задача: %s`, originalRequest, classification.URL, content, classification.Description)

	requestData := map[string]interface{}{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": systemPrompt,
			},
			{
				"role":    "user",
				"content": userPrompt,
			},
		},
		"stream":     false,
		"max_tokens": 2048,
	}

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return "", fmt.Errorf("ошибка создания JSON: %v", err)
	}

	proxyURL := "https://deproxy.kchugalinskiy.ru/deeproxy/api/completions"
	req, err := http.NewRequest("POST", proxyURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("41-2", "U0dMUjFs")

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка при выполнении запроса анализа: %v", err)
	}
	defer resp.Body.Close()

	responseBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ошибка от сервера анализа: %s", resp.Status)
	}

	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	err = json.Unmarshal(responseBytes, &apiResponse)
	if err != nil {
		return "", fmt.Errorf("ошибка парсинга JSON ответа анализа: %v", err)
	}

	if len(apiResponse.Choices) > 0 && apiResponse.Choices[0].Message.Content != "" {
		return fmt.Sprintf("Анализ сайта %s:\n\n%s", classification.URL, apiResponse.Choices[0].Message.Content), nil
	}

	return "Не удалось проанализировать содержимое сайта", nil
}

func (i *Interpreter) isCalculableExpression(input string) bool {
	input = strings.TrimSpace(input)

	if strings.HasPrefix(strings.ToLower(input), "curl ") {
		return false
	}

	if strings.Contains(input, "http://") || strings.Contains(input, "https://") || strings.Contains(input, "www.") {
		return false
	}

	if _, exists := i.variables[input]; exists {
		return true
	}

	hasMathOps := strings.ContainsAny(input, "+-*/")
	hasComparisonOps := strings.Contains(input, "==") || strings.Contains(input, "!=") ||
		strings.Contains(input, "<=") || strings.Contains(input, ">=") ||
		strings.Contains(input, "<") || strings.Contains(input, ">")

	if hasMathOps || hasComparisonOps {
		if i.containsURL(input) || i.containsFilePath(input) {
			return false
		}

		tokens := i.tokenizeExpression(input)
		hasNumbers := false
		hasValidVariables := false

		for _, token := range tokens {
			if i.isNumber(token) {
				hasNumbers = true
			}
			if val, exists := i.variables[token]; exists {
				if _, isNumber := val.(float64); isNumber {
					hasValidVariables = true
				}
			}
		}

		return hasNumbers || hasValidVariables
	}

	if _, err := strconv.ParseFloat(input, 64); err == nil {
		return true
	}
	if strings.Contains(input, "(") || strings.Contains(input, ")") {
		if !i.containsURL(input) && !i.containsFilePath(input) {
			return true
		}
	}

	return false
}

func (i *Interpreter) containsURL(input string) bool {
	return strings.Contains(input, "http://") || strings.Contains(input, "https://") ||
		strings.Contains(input, "www.") || strings.Contains(input, ".com") ||
		strings.Contains(input, ".ru") || strings.Contains(input, ".org")
}

func (i *Interpreter) containsFilePath(input string) bool {
	return strings.Contains(input, "/") && (strings.Contains(input, "curl") ||
		strings.Contains(input, "file") || strings.Contains(input, "dir"))
}

func (i *Interpreter) SendTextToDeepSeek(text string) (string, error) {
	systemPrompt := `Ты - помощник в калькуляторе. Отвечай точно и информативно на вопросы пользователя. 
Если вопрос требует развернутого ответа - дай его. Используй только проверенные факты и информацию.`
	requestData := map[string]interface{}{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": systemPrompt,
			},
			{
				"role":    "user",
				"content": text,
			},
		},
		"stream":     false,
		"max_tokens": 2048,
	}

	requestBody, err := json.Marshal(requestData)
	if err != nil {
		return "", fmt.Errorf("ошибка создания JSON: %v", err)
	}
	proxyURL := "https://deproxy.kchugalinskiy.ru/deeproxy/api/completions"
	req, err := http.NewRequest("POST", proxyURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("ошибка создания запроса: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("41-2", "U0dMUjFs")

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка при выполнении запроса к DeepSeek: %v", err)
	}
	defer resp.Body.Close()
	responseBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ошибка от сервера: %s, тело ответа: %s", resp.Status, string(responseBytes))
	}
	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	err = json.Unmarshal(responseBytes, &apiResponse)
	if err != nil {
		return "", fmt.Errorf("ошибка парсинга JSON ответа: %v", err)
	}

	if apiResponse.Error.Message != "" {
		return "", fmt.Errorf("ошибка от API: %s", apiResponse.Error.Message)
	}

	if len(apiResponse.Choices) > 0 && apiResponse.Choices[0].Message.Content != "" {
		return apiResponse.Choices[0].Message.Content, nil
	}

	return "Не удалось получить ответ от AI", nil
}

func (i *Interpreter) handleCurlAssignment(variable string, expression string) (interface{}, error) {
	result, err := i.handleCurlCommand(expression)
	if err != nil {
		return nil, err
	}
	i.variables[variable] = result
	return fmt.Sprintf("CURL результат сохранен в переменную '%s'", variable), nil
}

func (i *Interpreter) handleCurlCommand(input string) (string, error) {
	url := strings.TrimSpace(input[5:])
	if url == "" {
		return "", fmt.Errorf("curl: отсутствует URL")
	}

	url = strings.Trim(url, "\"")

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("curl: ошибка создания запроса: %v", err)
	}

	resp, err := i.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("curl: ошибка выполнения запроса: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("curl: ошибка чтения ответа: %v", err)
	}

	return string(body), nil
}

func (i *Interpreter) handleAssignment(variable string, expression string) (interface{}, error) {
	if strings.HasPrefix(strings.ToLower(expression), "curl ") {
		return i.handleCurlAssignment(variable, expression)
	}

	result, err := i.evaluateExpression(expression)
	if err != nil {
		return nil, err
	}

	i.variables[variable] = result
	return result, nil
}

func (i *Interpreter) evaluateExpression(expr string) (interface{}, error) {
	expr = strings.TrimSpace(expr)

	if val, ok := i.variables[expr]; ok {
		return val, nil
	}

	if i.containsStringVariables(expr) {
		return nil, fmt.Errorf("ошибка: выражение содержит строковые переменные, арифметические операции запрещены")
	}

	expr = i.replaceVariables(expr)

	for strings.Contains(expr, "(") {
		start := strings.LastIndex(expr, "(")
		end := strings.Index(expr[start:], ")") + start

		if end < start {
			return nil, fmt.Errorf("непарные скобки")
		}

		innerResult, err := i.evaluateExpression(expr[start+1 : end])
		if err != nil {
			return nil, err
		}

		if _, isString := innerResult.(string); isString {
			return nil, fmt.Errorf("нельзя использовать строки в арифметических операциях")
		}

		expr = expr[:start] + i.valueToString(innerResult) + expr[end+1:]
	}

	if result, handled, err := i.tryComparison(expr); handled {
		return result, err
	}

	result, err := i.evaluateArithmetic(expr)
	if err != nil {
		return nil, err 
	}

	return result, nil
}

func (i *Interpreter) containsStringVariables(expr string) bool {
	tokens := i.tokenizeExpression(expr)

	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if i.isOperator(token) || i.isNumber(token) {
			continue
		}
		if token == "true" || token == "false" {
			continue
		}
		if val, exists := i.variables[token]; exists {
			if _, isString := val.(string); isString {
				return true
			}
		}
	}
	return false
}

func (i *Interpreter) tryComparison(expr string) (interface{}, bool, error) {
	ops := []string{">=", "<=", "==", "!=", ">", "<"}

	for _, op := range ops {
		index := strings.Index(expr, op)
		if index != -1 {
			left := strings.TrimSpace(expr[:index])
			right := strings.TrimSpace(expr[index+len(op):])

			if i.containsStringVariables(left) || i.containsStringVariables(right) {
				return false, true, fmt.Errorf("операции сравнения со строковыми переменными запрещены")
			}

			leftVal, err := i.evaluateArithmetic(left)
			if err != nil {
				return false, true, err
			}

			rightVal, err := i.evaluateArithmetic(right)
			if err != nil {
				return false, true, err
			}

			result, err := i.compareValues(leftVal, rightVal, op)
			return result, true, err
		}
	}

	return nil, false, nil
}

func (i *Interpreter) evaluateArithmetic(expr string) (float64, error) {
	expr = strings.TrimSpace(expr)

	if num, err := strconv.ParseFloat(expr, 64); err == nil {
		return num, nil
	}

	for idx := len(expr) - 1; idx >= 0; idx-- {
		if expr[idx] == '+' || expr[idx] == '-' {
			if idx > 0 && !isArithmeticOperator(rune(expr[idx-1])) {
				left, err := i.evaluateArithmetic(expr[:idx])
				if err != nil {
					return 0, err
				}
				right, err := i.evaluateArithmetic(expr[idx+1:])
				if err != nil {
					return 0, err
				}

				if expr[idx] == '+' {
					return left + right, nil
				} else {
					return left - right, nil
				}
			}
		}
	}
	for idx := len(expr) - 1; idx >= 0; idx-- {
		if expr[idx] == '*' || expr[idx] == '/' {
			if idx > 0 && idx < len(expr)-1 {
				left, err := i.evaluateArithmetic(expr[:idx])
				if err != nil {
					return 0, err
				}
				right, err := i.evaluateArithmetic(expr[idx+1:])
				if err != nil {
					return 0, err
				}

				if expr[idx] == '*' {
					return left * right, nil
				} else {
					if right == 0 {
						return 0, fmt.Errorf("деление на ноль")
					}
					return left / right, nil
				}
			}
		}
	}

	return 0, fmt.Errorf("неизвестное выражение: %s", expr)
}

func isArithmeticOperator(char rune) bool {
	return char == '+' || char == '-' || char == '*' || char == '/' || char == '(' || char == ')'
}

func (i *Interpreter) compareValues(left, right float64, op string) (bool, error) {
	switch op {
	case "==":
		return left == right, nil
	case "!=":
		return left != right, nil
	case ">":
		return left > right, nil
	case "<":
		return left < right, nil
	case ">=":
		return left >= right, nil
	case "<=":
		return left <= right, nil
	default:
		return false, fmt.Errorf("неизвестная операция сравнения: %s", op)
	}
}

func (i *Interpreter) valueToString(value interface{}) string {
	switch v := value.(type) {
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (i *Interpreter) replaceVariables(expr string) string {
	tokens := i.tokenizeExpression(expr)

	for idx, token := range tokens {
		if i.isOperator(token) || i.isNumber(token) {
			continue
		}

		if val, exists := i.variables[token]; exists {
			tokens[idx] = i.valueToString(val)
		}
	}

	return strings.Join(tokens, "")
}

func (i *Interpreter) tokenizeExpression(expr string) []string {
	var tokens []string
	var currentToken strings.Builder

	for idx, char := range expr {
		if unicode.IsSpace(char) {
			if currentToken.Len() > 0 {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			}
			continue
		}

		if idx > 0 && (expr[idx-1:idx+1] == ":/" || (idx > 3 && expr[idx-3:idx+1] == "http")) {
			currentToken.WriteRune(char)
			continue
		}

		if i.isOperatorChar(char) {
			if currentToken.Len() > 0 {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			}
			tokens = append(tokens, string(char))
		} else {
			currentToken.WriteRune(char)
		}
	}

	if currentToken.Len() > 0 {
		tokens = append(tokens, currentToken.String())
	}

	return tokens
}

func (i *Interpreter) isOperatorChar(char rune) bool {
	return char == '+' || char == '-' || char == '*' || char == '/' ||
		char == '(' || char == ')' || char == '=' || char == '!' ||
		char == '<' || char == '>'
}

func (i *Interpreter) isOperator(token string) bool {
	return token == "+" || token == "-" || token == "*" || token == "/" ||
		token == "(" || token == ")" || token == "==" || token == "!=" ||
		token == "<" || token == ">" || token == "<=" || token == ">="
}

func (i *Interpreter) isNumber(token string) bool {
	_, err := strconv.ParseFloat(token, 64)
	return err == nil
}

func (i *Interpreter) GetHistory() []string {
	return i.historyRepo.GetLastCommands(10)
}

func isValidVariableName(name string) bool {
	if len(name) == 0 || !unicode.IsLetter(rune(name[0])) {
		return false
	}

	for _, char := range name {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			return false
		}
	}
	return true
}
func (i *Interpreter) AddSafeDirectory(dir string) (string, error) {
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("ошибка получения абсолютного пути: %v", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return "", fmt.Errorf("директория не существует: %s", absPath)
	}

	i.customSafeDirs = append(i.customSafeDirs, absPath)
	return fmt.Sprintf("Директория добавлена в безопасный список: %s", absPath), nil
}
