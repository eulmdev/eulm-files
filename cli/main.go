package main

import (
    "encoding/json"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
    "net/url"
    "os"
    "path/filepath"
    "strings"
    "time"

    "golang.org/x/term"
)

type File struct {
    Id         string `json:"id"`
    Name       string `json:"name"`
    UploadedAt string `json:"uploadedAt"`
    Creator    string `json:"creator"`
}

type UploadResponseBody struct {
    Id string `json:"id"`
}

type ListResponseBody struct {
    Files []File `json:"files"`
}

var args = parseArgs()
var client = &http.Client{Timeout: 30 * time.Second}

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
list: List all uploaded files
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

    fmt.Print("Enter your API key: ")
    apiKey, err := term.ReadPassword(int(os.Stdin.Fd()))
    if err != nil {
        fmt.Println("Error reading input")
        return
    }
    fmt.Println("[ENTERED]")

    pr, pw := io.Pipe()
    writer := multipart.NewWriter(pw)

    go func() {
        defer func(pw *io.PipeWriter) {
            if err = pw.Close(); err != nil {
                fmt.Println("Error closing pipe writer")
            }
        }(pw)
        defer func(writer *multipart.Writer) {
            if err = writer.Close(); err != nil {
                fmt.Println("Error closing multipart writer")
            }
        }(writer)

        part, err := writer.CreateFormFile("file", filepath.Base(filePath))
        if err != nil {
            if err = pw.CloseWithError(err); err != nil {
                fmt.Println("Error closing pipe writer")
            }
            fmt.Println("Error creating form file")
            return
        }

        if _, err := io.Copy(part, file); err != nil {
            if err = pw.CloseWithError(err); err != nil {
                fmt.Println("Error closing pipe writer")
            }
            fmt.Println("Error copying file")
            return
        }
    }()

    endpoint, err := url.JoinPath(apiUrl, "/upload")
    if err != nil {
        fmt.Println("Error constructing URL")
        return
    }

    req, err := http.NewRequest("POST", endpoint, pr)
    if err != nil {
        fmt.Println("Error creating request")
        return
    }
    req.Header.Add("Authorization", "Bearer "+string(apiKey))
    req.Header.Add("Content-Type", writer.FormDataContentType())

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

    if res.StatusCode == http.StatusCreated {
        var resBody UploadResponseBody
        if err = json.NewDecoder(res.Body).Decode(&resBody); err != nil {
            fmt.Println("Error parsing response body")
            return
        }
        fmt.Printf("File uploaded successfully to %s/%s\n", apiUrl, resBody.Id)
    } else if res.StatusCode == http.StatusUnauthorized {
        fmt.Println("Invalid API key or insufficient permissions")
    } else {
        fmt.Println("Error uploading file")
    }
}

func deleteCmd() {
    if len(args) < 2 {
        fmt.Println("The file ID is required")
        return
    }

    fileId := args[1]

    fmt.Print("Enter your API key: ")
    apiKey, err := term.ReadPassword(int(os.Stdin.Fd()))
    if err != nil {
        fmt.Println("Error reading input")
        return
    }
    fmt.Println("[ENTERED]")

    endpoint, err := url.JoinPath(apiUrl, "/"+fileId)
    if err != nil {
        fmt.Println("Error constructing URL")
        return
    }

    req, err := http.NewRequest("DELETE", endpoint, nil)
    if err != nil {
        fmt.Println("Error creating request")
        return
    }
    req.Header.Add("Authorization", "Bearer "+string(apiKey))

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

func listCmd() {
    fmt.Print("Enter your API key: ")
    apiKey, err := term.ReadPassword(int(os.Stdin.Fd()))
    if err != nil {
        fmt.Println("Error reading input")
        return
    }
    fmt.Println("[ENTERED]")

    endpoint, err := url.JoinPath(apiUrl, "/list")
    if err != nil {
        fmt.Println("Error constructing URL")
        return
    }

    req, err := http.NewRequest("GET", endpoint, nil)
    if err != nil {
        fmt.Println("Error creating request")
        return
    }
    req.Header.Add("Authorization", "Bearer "+string(apiKey))

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
        var resBody ListResponseBody
        if err = json.NewDecoder(res.Body).Decode(&resBody); err != nil {
            fmt.Println("Error parsing response body")
            return
        }

        if resBody.Files == nil {
            fmt.Println("No files found")
            return
        }

        nameLen := len("Name")
        urlLen := len("URL")
        creatorLen := len("Creator")
        uploadedLen := len("Uploaded at")
        for _, file := range resBody.Files {
            length := len(file.Name)
            if length > nameLen {
                nameLen = length
            }

            fileUrl, err := url.JoinPath(apiUrl, "/"+file.Id)
            if err != nil {
                fmt.Println("Error constructing URL")
            }
            length = len(fileUrl)
            if length > urlLen {
                urlLen = length
            }

            length = len(file.Creator)
            if length > creatorLen {
                creatorLen = length
            }

            length = len(file.UploadedAt)
            if length > uploadedLen {
                uploadedLen = length
            }
        }

        extendStr := func(s string, targetLength int) string {
            paddingLength := targetLength - len(s)
            padding := strings.Repeat(" ", paddingLength)
            return s + padding
        }

        fmt.Printf(
            "\n%s %s %s %s\n", extendStr("Name", nameLen), extendStr("Creator", creatorLen),
            extendStr("Uploaded at", uploadedLen), extendStr("URL", urlLen),
        )

        for _, file := range resBody.Files {
            fileUrl, err := url.JoinPath(apiUrl, "/"+file.Id)
            if err != nil {
                fmt.Println("Error constructing URL")
            }
            fmt.Printf(
                "%s %s %s %s\n", extendStr(file.Name, nameLen), extendStr(file.Creator, creatorLen),
                extendStr(file.UploadedAt, uploadedLen), extendStr(fileUrl, urlLen),
            )
        }

        fmt.Println()
    } else if res.StatusCode == http.StatusUnauthorized {
        fmt.Println("Invalid API key or insufficient permissions")
    } else {
        fmt.Println("Error fetching files")
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
    } else if args[0] == "list" {
        listCmd()
    } else {
        fmt.Println("Unknown command (maybe try `help` instead)")
    }
}
