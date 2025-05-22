package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type UploadResponseBody struct {
	Id string `json:"id"`
}

var args = parseArgs()
var version = "v1.0.0"
var apiUrl = "https://files.eulm.dev"

func parseArgs() []string {
	var validArgs []string

	for _, arg := range os.Args[1:] {
		if !strings.HasPrefix(arg, "-") {
			validArgs = append(validArgs, arg)
		}
	}

	return validArgs
}

func printHelp() {
	fmt.Printf(`
  Eulm Files CLI %s

  help: Display this help page
  version: Display the CLI version
  upload [file path]: Upload a file from its path
  delete [file ID]: Delete a file from its ID
    `+"\n", version)
}

func uploadCmd() {
	if len(args) < 2 {
		fmt.Println("The file path is required")
		return
	}

	filePath := args[1]

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Println("Invalid file path - the file doesn't exist")
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening file")
		return
	}
	defer func(file *os.File) {
		if err = file.Close(); err != nil {
			fmt.Println("Error closing file")
		}
	}(file)

	reqBody := &bytes.Buffer{}
	writer := multipart.NewWriter(reqBody)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		fmt.Println("Error creating form file")
		return
	}
	if _, err = io.Copy(part, file); err != nil {
		fmt.Println("Error copying file")
		return
	}

	if err = writer.Close(); err != nil {
		fmt.Println("Error closing multipart writer")
		return
	}

	var apiKey string
	fmt.Println("Enter your API key (Enter to submit)")
	if _, err = fmt.Scanln(&apiKey); err != nil {
		fmt.Println("Error reading input")
		return
	}

	req, err := http.NewRequest("POST", apiUrl+"/upload", reqBody)
	if err != nil {
		fmt.Println("Error creating request")
		return
	}
	req.Header.Add("Authorization", "Bearer "+apiKey)
	req.Header.Add("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request")
		return
	}
	defer func(file *os.File) {
		if err = res.Body.Close(); err != nil {
			fmt.Println("Error closing response body")
		}
	}(file)

	var resBody UploadResponseBody
	if err = json.NewDecoder(res.Body).Decode(&resBody); err != nil {
		fmt.Println("Error parsing response body")
		return
	}

	if res.StatusCode != http.StatusCreated {
		fmt.Println("Error uploading file")
	} else {
		fmt.Printf("File uploaded successfully to %s/%s\n", apiUrl, resBody.Id)
	}
}

func deleteCmd() {
	if len(args) < 2 {
		fmt.Println("The file ID is required")
		return
	}

	fileId := args[1]

	var apiKey string
	fmt.Println("Enter your API key (Enter to submit)")
	if _, err := fmt.Scanln(&apiKey); err != nil {
		fmt.Println("Error reading input")
		return
	}

	req, err := http.NewRequest("DELETE", apiUrl+"/"+fileId, nil)
	if err != nil {
		fmt.Println("Error creating request")
		return
	}
	req.Header.Add("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request")
		return
	}
	defer func(res *http.Response) {
		if err = res.Body.Close(); err != nil {
			fmt.Println("Error closing response body")
		}
	}(res)

	if res.StatusCode == http.StatusOK {
		fmt.Println("File deleted successfully")
	} else if res.StatusCode == http.StatusNotFound {
		fmt.Println("File not found")
	} else if res.StatusCode == http.StatusUnauthorized {
		fmt.Println("Invalid API key or insufficient permissions")
	} else {
		fmt.Println("Error deleting file")
	}
}

func main() {
	if len(args) == 0 || args[0] == "help" {
		printHelp()
	} else if args[0] == "version" {
		fmt.Println("Eulm Files CLI " + version)
	} else if args[0] == "upload" {
		uploadCmd()
	} else if args[0] == "delete" {
		deleteCmd()
	} else {
		fmt.Println("Unknown command (maybe try `help` instead)")
	}
}
