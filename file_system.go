package patriotsfs

import (
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	pecho "github.com/Gerardo115pp/patriots_lib/echo"
)

type PatriotsFs struct {
	is_readonly      bool
	directories      map[string]*PatriotsFsDirectory // route => directory's first level.. recursive support will come later
	prefix           string
	middleware       []Middleware
	max_size         int64
	permissions_mode os.FileMode
}

// apply middleware to the http request
func (patriot_fs *PatriotsFs) applyMiddleWare(handler http.HandlerFunc) http.HandlerFunc {
	for _, middleware := range patriot_fs.middleware {
		handler = middleware(handler)
	}
	return handler
}

func (patriot_fs *PatriotsFs) activate() {
	go func() {
		for {
			for _, fs_directory := range patriot_fs.directories {
				fs_directory.verifyContentIntegrity()
			}
			time.Sleep(time.Second * 5)
		}
	}()
}

func (patriot_fs *PatriotsFs) AddMiddleware(middleware Middleware) {
	patriot_fs.middleware = append(patriot_fs.middleware, middleware)
}

func (patriot_fs *PatriotsFs) AddDirectory(route string, directory_path string) error {
	if _, exists := patriot_fs.directories[directory_path]; !exists {
		// validating route, must be a string with no spaces, starting with a / and no trailing /. if not, it will be added with a /
		route = strings.TrimSpace(route)
		if !strings.HasPrefix(route, "/") {
			route = "/" + route
		}
		route = strings.TrimSuffix(route, "/")
		if strings.Contains(route, " ") {
			fmt.Println("Cant add route with spaces:", route)
			os.Exit(1)
		}

		new_directory, err := CreateNewFsDirectory(directory_path)
		if err != nil {
			return err
		}
		patriot_fs.directories[route] = new_directory
		return nil
	} else {
		return fmt.Errorf("Trying to add an exisiting diectory '%s' which already exists. if this is intentional please use RedefineDirectory instead", directory_path)
	}
}

func (patriot_fs *PatriotsFs) GetPrefix() string {
	return patriot_fs.prefix
}

func (patriot_fs *PatriotsFs) GetDirectoryFromRequest(request *http.Request) (*PatriotsFsDirectory, error) {
	var request_path = strings.TrimPrefix(request.URL.Path, patriot_fs.prefix)
	for route, directory := range patriot_fs.directories {
		if strings.HasPrefix(request_path, route) {
			return directory, nil
		}
	}
	return nil, fmt.Errorf("No directory found for route '%s'", request_path)
}

func (patriot_fs *PatriotsFs) RedefineDirectory(route string, directory_path string) error {
	if _, exists := patriot_fs.directories[directory_path]; exists {
		err := patriot_fs.directories[route].rebase(directory_path)
		pecho.Echo(pecho.BlueFG, fmt.Sprintf("redefine route %s to directory '%s'", route, directory_path))
		return err
	} else {
		return fmt.Errorf("Trying to redefine a directory '%s' which does not exist. first add it with AddDirectory", directory_path)
	}
}

func (patriot_fs *PatriotsFs) returnFile(response http.ResponseWriter, request *http.Request) {
	var directory *PatriotsFsDirectory
	directory, err := patriot_fs.GetDirectoryFromRequest(request)
	if err != nil {
		fmt.Println(err.Error())
		response.WriteHeader(http.StatusNotFound)
		return
	}
	var filename string = strings.Split(request.URL.Path, directory.BaseName)[1]
	if filename != "" {
		file_descriptor, err := os.Open(directory.GetFileAbsoultPath(filename))
		if err != nil {
			pecho.EchoErr(err)
			response.WriteHeader(500)
		}
		defer file_descriptor.Close()
		file_header := make([]byte, 512)
		file_descriptor.Read(file_header)

		Content_Type := http.DetectContentType(file_header)

		FileStat, err := file_descriptor.Stat()
		if err != nil {
			pecho.EchoErr(err)
			response.WriteHeader(500)
		}
		var file_size string = strconv.FormatInt(FileStat.Size(), 10)

		response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		response.Header().Set("Content-Type", Content_Type)
		response.Header().Set("Content-Length", file_size)

		file_descriptor.Seek(0, 0)
		var filedata []byte
		filedata, err = ioutil.ReadAll(file_descriptor)
		if err != nil {
			pecho.EchoErr(err)
			response.WriteHeader(500)
		}
		response.Write(filedata)

	} else {
		response.WriteHeader(400)
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		response.Write([]byte("Bad request"))
	}
}

func (patriot_fs *PatriotsFs) saveFile(response http.ResponseWriter, request *http.Request) {
	var directory *PatriotsFsDirectory
	directory, err := patriot_fs.GetDirectoryFromRequest(request)
	if err != nil {
		fmt.Println(err.Error())
		response.WriteHeader(http.StatusNotFound)
		return
	}
	var file_data multipart.File
	var file_header *multipart.FileHeader

	//extraction of the file data
	file_data, file_header, err = request.FormFile("file")
	if err != nil {
		pecho.EchoErr(err)
		response.WriteHeader(400)
		return
	}

	//extraction of the file information
	splitted_path := strings.Split(request.URL.Path, fmt.Sprintf("%s/", directory.BaseName))
	var path string = ""
	if len(splitted_path) > 1 {
		path = splitted_path[len(splitted_path)-1]
	} else {
		// check that the file is not larger than max_size
		if file_header.Size <= patriot_fs.max_size && file_header.Size > 0 {
			var file []byte
			file, err = ioutil.ReadAll(file_data)
			if err != nil {
				pecho.EchoErr(err)
				response.WriteHeader(400)
				return
			}

			// write the file
			err := directory.WriteFile(path+file_header.Filename, file, patriot_fs.permissions_mode)
			if err != nil {
				pecho.EchoErr(err)
				response.WriteHeader(500)
				return
			}

			response.WriteHeader(200)
		} else if file_header.Size == 0 {
			response.WriteHeader(http.StatusLengthRequired)
			response.Write([]byte("No empty files allowed"))
			return
		} else {
			response.WriteHeader(http.StatusRequestEntityTooLarge)
		}
	}
}

// prefix should be a string with no spaces, starting with a / and no trailing /
func (patriot_fs *PatriotsFs) SetPrefix(prefix string) {
	patriot_fs.prefix = prefix

	//cleaning the prefix
	patriot_fs.prefix = strings.TrimSpace(patriot_fs.prefix)
	if !strings.HasPrefix(patriot_fs.prefix, "/") {
		patriot_fs.prefix = "/" + patriot_fs.prefix
	}
	patriot_fs.prefix = strings.TrimSuffix(patriot_fs.prefix, "/")

}

func (patriot_fs *PatriotsFs) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	var handler http.HandlerFunc
	switch request.Method {
	case "GET":
		handler = patriot_fs.applyMiddleWare(patriot_fs.returnFile)
		handler(response, request)
	case "POST":
		handler = patriot_fs.applyMiddleWare(patriot_fs.saveFile)
		handler(response, request)
	default:
		response.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func CreateFs(readonly bool, max_size int64) *PatriotsFs {
	var new_fs *PatriotsFs = new(PatriotsFs)
	new_fs.directories = make(map[string]*PatriotsFsDirectory)
	new_fs.middleware = make([]Middleware, 0)
	new_fs.is_readonly = readonly
	new_fs.max_size = max_size
	new_fs.permissions_mode = 0644
	return new_fs
}
