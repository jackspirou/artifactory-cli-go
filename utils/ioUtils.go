package utils

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackspirou/syscerts"
)

var tempDirPath string

func GetFileNameFromPath(path string) string {
	index := strings.LastIndex(path, "/")
	if index != -1 {
		return path[index+1:]
	}
	index = strings.LastIndex(path, "\\")
	if index != -1 {
		return path[index+1:]
	}
	return path
}

func IsDir(path string) bool {
	if !IsPathExists(path) {
		return false
	}
	f, err := os.Stat(path)
	CheckError(err)
	return f.IsDir()
}

func IsPathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func IsFileExists(path string) bool {
	if !IsPathExists(path) {
		return false
	}
	f, err := os.Stat(path)
	CheckError(err)
	return !f.IsDir()
}

func IsDirExists(path string) bool {
	if !IsPathExists(path) {
		return false
	}
	f, err := os.Stat(path)
	CheckError(err)
	return f.IsDir()
}

func ReadFile(filePath string) []byte {
	content, err := ioutil.ReadFile(filePath)
	CheckError(err)
	return content
}

func UploadFile(f *os.File, url string, artifactoryDetails ArtifactoryDetails, details *FileDetails) *http.Response {
	if details == nil {
		details = GetFileDetails(f.Name())
	}
	req, err := http.NewRequest("PUT", url, f)
	CheckError(err)
	req.ContentLength = details.Size
	req.Close = true

	addAuthHeaders(req, artifactoryDetails)
	addUserAgentHeader(req)

	size := strconv.FormatInt(details.Size, 10)

	req.Header.Set("Content-Length", size)
	req.Header.Set("X-Checksum-Sha1", details.Sha1)
	req.Header.Set("X-Checksum-Md5", details.Md5)

	// Add CA certs
	certpool := syscerts.SystemRootsPool()
	tlsConfig := &tls.Config{RootCAs: certpool}
	transport := &http.Transport{TLSClientConfig: tlsConfig}

	client := &http.Client{Transport: transport}
	resp, err := client.Do(req)
	CheckError(err)
	defer resp.Body.Close()
	return resp
}

func DownloadFile(downloadPath, localPath, fileName string, flat bool, artifactoryDetails ArtifactoryDetails) *http.Response {
	if !flat && localPath != "" {
		os.MkdirAll(localPath, 0777)
		fileName = localPath + "/" + fileName
	}

	out, err := os.Create(fileName)
	CheckError(err)
	defer out.Close()
	resp, body := SendGet(downloadPath, nil, artifactoryDetails)
	out.Write(body)
	CheckError(err)
	return resp
}

func SendPut(url string, content []byte, headers map[string]string, artifactoryDetails ArtifactoryDetails) (*http.Response, []byte) {
	return Send("PUT", url, content, headers, artifactoryDetails)
}

func SendPost(url string, content []byte, artifactoryDetails ArtifactoryDetails) (*http.Response, []byte) {
	return Send("POST", url, content, nil, artifactoryDetails)
}

func SendGet(url string, headers map[string]string, artifactoryDetails ArtifactoryDetails) (*http.Response, []byte) {
	return Send("GET", url, nil, headers, artifactoryDetails)
}

func SendHead(url string, artifactoryDetails ArtifactoryDetails) (*http.Response, []byte) {
	return Send("HEAD", url, nil, nil, artifactoryDetails)
}

func Send(method string, url string, content []byte, headers map[string]string,
	artifactoryDetails ArtifactoryDetails) (*http.Response, []byte) {

	var req *http.Request
	var err error

	if content != nil {
		req, err = http.NewRequest(method, url, bytes.NewBuffer(content))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	CheckError(err)
	req.Close = true

	addAuthHeaders(req, artifactoryDetails)
	addUserAgentHeader(req)
	if headers != nil {
		for name := range headers {
			req.Header.Set(name, headers[name])
		}
	}

	// Add CA certs
	certpool := syscerts.SystemRootsPool()
	tlsConfig := &tls.Config{RootCAs: certpool}
	transport := &http.Transport{TLSClientConfig: tlsConfig}

	client := &http.Client{Transport: transport}
	resp, err := client.Do(req)
	CheckError(err)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	return resp, body
}

// Return the recursive list of files and directories in the specified path
func ListFilesRecursive(path string) []string {
	fileList := []string{}
	err := filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		fileList = append(fileList, path)
		return nil
	})
	CheckError(err)
	return fileList
}

// Return the list of files and directories in the specified path
func ListFiles(path string) []string {
	sep := GetFileSeperator()
	if !strings.HasSuffix(path, sep) {
		path += sep
	}
	fileList := []string{}
	files, _ := ioutil.ReadDir(path)

	for _, f := range files {
		filePath := path + f.Name()
		if IsFileExists(filePath) {
			fileList = append(fileList, filePath)
		}
	}
	return fileList
}

func GetTempDirPath() string {
	if tempDirPath == "" {
		Exit(ExitCodeError, "Function cannot be used before 'tempDirPath' is created.")
	}
	return tempDirPath
}

func CreateTempDirPath() {
	if tempDirPath != "" {
		Exit(ExitCodeError, "'tempDirPath' has already been initialized.")
	}
	path, err := ioutil.TempDir("", "artifactory.cli.")
	CheckError(err)
	tempDirPath = path
}

func RemoveTempDir() {
	if IsDirExists(tempDirPath) {
		os.RemoveAll(tempDirPath)
	}
}

// Reads the content of the file in the source path and appends it to
// the file in the destination path.
func AppendFile(srcPath string, destFile *os.File) {
	srcFile, err := os.Open(srcPath)
	CheckError(err)

	defer func() {
		err := srcFile.Close()
		CheckError(err)
	}()

	reader := bufio.NewReader(srcFile)

	writer := bufio.NewWriter(destFile)
	buf := make([]byte, 1024000)
	for {
		n, err := reader.Read(buf)
		if err != io.EOF {
			CheckError(err)
		}
		if n == 0 {
			break
		}
		_, err = writer.Write(buf[:n])
		CheckError(err)
	}
	err = writer.Flush()
	CheckError(err)
}

func addAuthHeaders(req *http.Request, artifactoryDetails ArtifactoryDetails) {
	if artifactoryDetails.SshAuthHeaders != nil {
		for name := range artifactoryDetails.SshAuthHeaders {
			req.Header.Set(name, artifactoryDetails.SshAuthHeaders[name])
		}
	} else if artifactoryDetails.User != "" && artifactoryDetails.Password != "" {
		req.SetBasicAuth(artifactoryDetails.User, artifactoryDetails.Password)
	}
}

func addUserAgentHeader(req *http.Request) {
	req.Header.Set("User-Agent", "artifactory-cli-go/"+GetVersion())
}
