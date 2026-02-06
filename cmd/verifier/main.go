package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

var (
	baseURL  string
	username string
	password string
	token    string
	homeDir  string
)

type Response struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type FilesListData struct {
	Path     string `json:"path"`
	Total    int    `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

func main() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("=== CloudQue System Verification Tool ===")

	// 1. Get Configuration
	fmt.Print("Enter Base URL [http://localhost:8080]: ")
	input, _ := reader.ReadString('\n')
	baseURL = strings.TrimSpace(input)
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	fmt.Print("Enter Test Username (must be a valid Linux user on server): ")
	input, _ = reader.ReadString('\n')
	username = strings.TrimSpace(input)

	fmt.Print("Enter Test Password (SSH password): ")
	input, _ = reader.ReadString('\n')
	password = strings.TrimSpace(input)

	if username == "" || password == "" {
		fmt.Println("Error: Username and Password are required.")
		return
	}

	fmt.Println("\nStarting Verification...")

	// 2. Test Registration (Create DB record)
	testRegister()

	// 3. Test Login (Verify SSH & Get Token)
	if !testLogin() {
		return
	}

	// 4. Test File List (SFTP)
	if !testFileList() {
		return
	}

	testFileUpload()
	testFileDownload()
	testFileDelete()

	// 5. Test Terminal (WebSocket)
	testTerminal()

	fmt.Println("\n=== Verification Complete ===")
}

func testRegister() {
	fmt.Println("\n[Step 1] Testing Registration...")
	url := baseURL + "/api/auth/register"
	data := map[string]string{
		"username": username,
		"password": password,
		"email":    username + "@example.com",
	}
	jsonData, _ := json.Marshal(data)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("❌ Registration Request Failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var r Response
	json.Unmarshal(body, &r)

	// Code 0 is success in this system
	if resp.StatusCode == 200 && r.Code == 0 {
		fmt.Println("✅ Registration Successful")
	} else if r.Code == 40003 || r.Code == 1002 { // User already exists
		fmt.Println("✅ User already registered (Expected)")
	} else {
		fmt.Printf("❌ Registration Failed: %s\n", string(body))
	}
}

func testLogin() bool {
	fmt.Println("\n[Step 2] Testing Login...")
	url := baseURL + "/api/auth/login"
	data := map[string]string{
		"username": username,
		"password": password,
	}
	jsonData, _ := json.Marshal(data)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("❌ Login Request Failed: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var r Response
	json.Unmarshal(body, &r)

	if resp.StatusCode != 200 || r.Code != 0 {
		fmt.Printf("❌ Login Failed: %s\n", string(body))
		return false
	}

	var loginData LoginResponse
	json.Unmarshal(r.Data, &loginData)
	token = loginData.Token

	if token == "" {
		fmt.Println("❌ Login Success but Token is empty")
		return false
	}

	fmt.Println("✅ Login Successful. Token received.")
	return true
}

func testFileList() bool {
	fmt.Println("\n[Step 3] Testing File List (SFTP)...")
	// Use path=. to list current directory (home)
	reqURL := baseURL + "/api/files/list?path=.&page=1&page_size=100"
	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ File List Request Failed: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var r Response
	json.Unmarshal(body, &r)

	if resp.StatusCode == 200 && r.Code == 0 {
		fmt.Println("✅ File List Successful. SFTP connection is working.")

		var data FilesListData
		if err := json.Unmarshal(r.Data, &data); err != nil {
			fmt.Printf("⚠️ Failed to parse file list data: %v\n", err)
		} else {
			homeDir = data.Path
			fmt.Printf("   Current Path (Home): %s\n", homeDir)
			fmt.Printf("   Pagination: Total=%d, Page=%d, Size=%d\n", data.Total, data.Page, data.PageSize)
		}
		return true
	} else {
		fmt.Printf("❌ File List Failed: %s\n", string(body))
		return false
	}
}

func testFileUpload() {
	fmt.Println("\n[Step 3.1] Testing File Upload (SFTP)...")

	if homeDir == "" {
		fmt.Println("⚠️ Skipping Upload: Home directory not found from previous step.")
		return
	}

	// Create a dummy file content
	content := []byte("Hello, CloudQue! This is a test file.")
	fileName := "test_upload.txt"
	targetPath := homeDir // Use the detected home directory

	// Prepare multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		fmt.Printf("❌ CreateFormFile Failed: %v\n", err)
		return
	}
	part.Write(content)

	_ = writer.WriteField("target_path", targetPath)

	err = writer.Close()
	if err != nil {
		fmt.Printf("❌ Writer Close Failed: %v\n", err)
		return
	}

	reqURL := baseURL + "/api/files/upload_file"
	req, _ := http.NewRequest("POST", reqURL, body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Upload Request Failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var r Response
	json.Unmarshal(respBody, &r)

	if resp.StatusCode == 200 && r.Code == 0 {
		fmt.Println("✅ File Upload Successful.")
	} else {
		fmt.Printf("❌ File Upload Failed: %s\n", string(respBody))
	}
}

func testFileDownload() {
	fmt.Println("\n[Step 3.2] Testing File Download (SFTP)...")

	if homeDir == "" {
		fmt.Println("⚠️ Skipping Download: Home directory not found.")
		return
	}

	// Must match what we uploaded
	filePath := homeDir + "/test_upload.txt"
	if strings.HasSuffix(homeDir, "/") {
		filePath = homeDir + "test_upload.txt"
	}

	reqURL := baseURL + "/api/files/download?path=" + filePath
	req, _ := http.NewRequest("GET", reqURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Download Request Failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("❌ Download Failed (Status %d): %s\n", resp.StatusCode, string(body))
		return
	}

	content, _ := io.ReadAll(resp.Body)
	expected := "Hello, CloudQue! This is a test file."
	if string(content) == expected {
		fmt.Println("✅ File Download Successful & Content Verified.")
	} else {
		fmt.Printf("❌ File Download Content Mismatch. Got: %s\n", string(content))
	}
}

func testFileDelete() {
	fmt.Println("\n[Step 3.3] Testing File Delete (SFTP)...")

	if homeDir == "" {
		fmt.Println("⚠️ Skipping Delete: Home directory not found.")
		return
	}

	filePath := homeDir + "/test_upload.txt"
	if strings.HasSuffix(homeDir, "/") {
		filePath = homeDir + "test_upload.txt"
	}

	data := map[string]string{
		"path": filePath,
	}
	jsonData, _ := json.Marshal(data)

	reqURL := baseURL + "/api/files/delete"
	req, _ := http.NewRequest("DELETE", reqURL, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Delete Request Failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var r Response
	json.Unmarshal(body, &r)

	if resp.StatusCode == 200 && r.Code == 0 {
		fmt.Println("✅ File Delete Successful.")
	} else {
		fmt.Printf("❌ File Delete Failed: %s\n", string(body))
	}
}

func testTerminal() {
	fmt.Println("\n[Step 4] Testing Terminal (WebSocket)...")

	// Convert http:// to ws://
	// Path should match router: /api/terminal/ws
	wsURL := strings.Replace(baseURL, "http", "ws", 1) + "/api/terminal/ws?token=" + token

	header := http.Header{}
	// header.Add("Authorization", "Bearer "+token) // Usually sent in query param for WS

	c, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		fmt.Printf("❌ WebSocket Connection Failed: %v\n", err)
		return
	}
	defer c.Close()

	fmt.Println("✅ WebSocket Connected")

	// Send a simple command: "ls\r"
	// Protocol might expect specific format. Assuming raw text or JSON.
	// Based on xterm.js usually it's just raw bytes or a specific JSON message.
	// Let's try sending a simple string if the backend expects text.
	// Looking at code (not shown fully but assuming standard)

	// Sending "ls -la\n"
	err = c.WriteMessage(websocket.TextMessage, []byte("ls -la\r"))
	if err != nil {
		fmt.Printf("❌ WebSocket Write Failed: %v\n", err)
		return
	}

	// Read response
	_, message, err := c.ReadMessage()
	if err != nil {
		fmt.Printf("❌ WebSocket Read Failed: %v\n", err)
		return
	}
	fmt.Printf("✅ WebSocket Response: %s\n", string(message))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
