package main

import (
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

type PermissionLevel int

type File struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	UploadedAt string `json:"uploadedAt"`
	Creator    string `json:"creator"`
}

const (
	NoPerms PermissionLevel = iota
	ReadWriteSelf
	ReadWriteAll
	Administrator
)

func validatePerms(requiredPerms PermissionLevel, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")

		var perms PermissionLevel
		var username string
		if err := db.QueryRow("SELECT permissions, username FROM users WHERE api_key = ?", apiKey).Scan(&perms, &username); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				respondJSON(w, http.StatusUnauthorized, map[string]any{"message": "Invalid API key"})
				return
			}
			respondJSON(w, http.StatusInternalServerError, map[string]any{"message": "An unexpected error occurred"})
			logger.Error("Error querying permissions from API key:", err.Error())
			return
		}

		if perms < requiredPerms {
			respondJSON(w, http.StatusUnauthorized, map[string]any{"message": "Insufficient permissions"})
			return
		}

		r.Header.Set("username", username)
		r.Header.Set("permissions", strconv.Itoa(int(perms)))

		next.ServeHTTP(w, r)
	}
}

func newFileId() (string, error) {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

	result := make([]byte, 8)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	resultStr := string(result)

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM files WHERE id = ?", resultStr).Scan(&count); err != nil {
		return "", err
	}

	if count > 0 {
		return newFileId()
	}
	return resultStr, nil
}

func getPermissions(r *http.Request) (PermissionLevel, error) {
	perms, err := strconv.Atoi(r.Header.Get("permissions"))
	if err != nil {
		return NoPerms, err
	}
	return PermissionLevel(perms), nil
}

func handleApi(r *mux.Router) {
	r.HandleFunc("/upload", validatePerms(ReadWriteSelf, func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 500*1024*1024) // 500MB limit

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]any{"message": "Invalid multipart form data"})
			logger.Warn("Upload failed - malformed form data:", err.Error())
			return
		}

		file, handler, err := r.FormFile("file")
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]any{"message": "Missing file in request"})
			logger.Warn("Upload failed - missing file field:", err.Error())
			return
		}
		defer func(file multipart.File) {
			if err = file.Close(); err != nil {
				logger.Error("Error closing uploaded file:", err.Error())
			}
		}(file)

		fileId, err := newFileId()
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]any{"message": "An unexpected error occurred"})
			logger.Error("Error generating file ID:", err.Error())
		}

		username := r.Header.Get("username")
		_, err = db.Exec("INSERT INTO files (id, file_name, uploaded_at, creator) VALUES (?, ?, datetime('now'), ?)", fileId, handler.Filename, username)
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]any{"message": "An unexpected error occurred"})
			logger.Error("Error inserting file into database:", err.Error())
			return
		}

		filePath := fmt.Sprintf("db/%s.dat", fileId)
		outFile, err := os.Create(filePath)
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]any{"message": "An unexpected error occurred"})
			logger.Error(fmt.Sprintf("Error creating file %s:", filePath), err.Error())
			return
		}
		defer func(file *os.File) {
			if err = file.Close(); err != nil {
				logger.Error(fmt.Sprintf("Error closing file %s:", filePath), err.Error())
			}
		}(outFile)

		if _, err = file.Seek(0, 0); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]any{"message": "An unexpected error occurred"})
			logger.Error("Error resetting file stream:", err.Error())
			return
		}

		if _, err = outFile.ReadFrom(file); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]any{"message": "An unexpected error occurred"})
			logger.Error("Error writing to file:", err.Error())
			return
		}

		logger.Info(fmt.Sprintf("File %s uploaded by %s", fileId, username))
		respondJSON(w, http.StatusCreated, map[string]any{
			"message": "File uploaded successfully",
			"id":      fmt.Sprint(fileId),
		})
	})).Methods("POST")

	r.HandleFunc("/list", validatePerms(ReadWriteSelf, func(w http.ResponseWriter, r *http.Request) {
		perms, err := getPermissions(r)
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]any{"message": "An unexpected error occurred"})
			logger.Warn("Error parsing permissions header:", err.Error())
			return
		}

		var rows *sql.Rows

		if perms < ReadWriteAll {
			username := r.Header.Get("username")
			rows, err = db.Query("SELECT id, file_name, uploaded_at, creator FROM files WHERE creator = ?", username)
			if err != nil {
				respondJSON(w, http.StatusInternalServerError, map[string]any{"message": "An unexpected error occurred"})
				logger.Warn("Error querying files from creator:", err.Error())
				return
			}
		} else {
			rows, err = db.Query("SELECT id, file_name, uploaded_at, creator FROM files")
			if err != nil {
				respondJSON(w, http.StatusInternalServerError, map[string]any{"message": "An unexpected error occurred"})
				logger.Warn("Error querying all files:", err.Error())
				return
			}
		}
		defer func(rows *sql.Rows) {
			if err = rows.Close(); err != nil {
				logger.Error("Error closing queried rows:", err.Error())
			}
		}(rows)

		var files []File
		for rows.Next() {
			var file File
			err = rows.Scan(&file.Id, &file.Name, &file.UploadedAt, &file.Creator)
			if err != nil {
				respondJSON(w, http.StatusInternalServerError, map[string]any{"message": "An unexpected error occurred"})
				logger.Warn("Error reading queried row:", err.Error())
				return
			}
			files = append(files, file)
		}

		respondJSON(w, http.StatusOK, map[string]any{
			"message": "Files fetched successfully",
			"files":   files,
		})
	})).Methods("GET")

	r.HandleFunc("/{fileId}", func(w http.ResponseWriter, r *http.Request) {
		var err error

		fileId := mux.Vars(r)["fileId"]

		var fileName string
		if err = db.QueryRow("SELECT file_name FROM files WHERE id = ?", fileId).Scan(&fileName); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				respondJSON(w, http.StatusNotFound, map[string]any{"message": "File not found"})
				return
			}
			respondJSON(w, http.StatusInternalServerError, map[string]any{"message": "An unexpected error occurred"})
			logger.Error("Error querying file from ID:", err.Error())
			return
		}

		var fileData []byte
		filePath := fmt.Sprintf("db/%s.dat", fileId)
		if fileData, err = os.ReadFile(filePath); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]any{"message": "An unexpected error occurred"})
			logger.Error(fmt.Sprintf("Error reading file %s:", filePath), err.Error())
			return
		}

		respondFile(w, http.StatusOK, fileName, fileData)
	}).Methods("GET")

	r.HandleFunc("/{fileId}", validatePerms(ReadWriteSelf, func(w http.ResponseWriter, r *http.Request) {
		var err error

		fileId := mux.Vars(r)["fileId"]
		username := r.Header.Get("username")

		perms, err := getPermissions(r)
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]any{"message": "An unexpected error occurred"})
			logger.Warn("Error parsing permissions header:", err.Error())
			return
		}

		if perms < ReadWriteAll {
			var fileUser string
			if err = db.QueryRow("SELECT creator FROM files WHERE id = ?", fileId).Scan(&fileUser); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					respondJSON(w, http.StatusNotFound, map[string]any{"message": "File not found"})
					return
				}
				respondJSON(w, http.StatusInternalServerError, map[string]any{"message": "An unexpected error occurred"})
				logger.Error("Error querying file creator from ID:", err.Error())
				return
			}

			if username != fileUser {
				respondJSON(w, http.StatusUnauthorized, map[string]any{"message": "Insufficient permissions"})
				return
			}
		}

		var count int
		if err = db.QueryRow("SELECT COUNT(*) FROM files WHERE id = ?", fileId).Scan(&count); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]any{"message": "An unexpected error occurred"})
			logger.Error("Error counting files with ID:", err.Error())
			return
		}
		if count == 0 {
			respondJSON(w, http.StatusNotFound, map[string]any{"message": "File not found"})
			return
		}

		if err = db.QueryRow("DELETE FROM files WHERE id = ?", fileId).Scan(); err != nil && !errors.Is(err, sql.ErrNoRows) {
			respondJSON(w, http.StatusInternalServerError, map[string]any{"message": "An unexpected error occurred"})
			logger.Error("Error deleting file from ID:", err.Error())
			return
		}

		filePath := fmt.Sprintf("db/%s.dat", fileId)
		if err = os.Remove(filePath); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]any{"message": "An unexpected error occurred"})
			logger.Error(fmt.Sprintf("Error deleting file %s:", filePath), err.Error())
			return
		}

		logger.Info(fmt.Sprintf("File %s deleted by %s", fileId, username))
		respondJSON(w, http.StatusOK, map[string]any{"message": "File deleted successfully"})
	})).Methods("DELETE")
}
