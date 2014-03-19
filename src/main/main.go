package main

import (
    "fmt"
	"html/template"
    "io/ioutil"
    "os"
    "io"
	"path/filepath"
	"archive/zip"
    "net/http"
	"strings"
	"encoding/json"
	"log"
	"github.com/Unknwon/goconfig"
)

type Course struct {
	Name string `json:"coursename"`
}

type CourseFile struct {
	Name string `json:"name"`
	Add  string `json:"add"`
	Type string `json:"type"`
	Adds []string `json:"adds"`
}

var dataDir string = "../data/"
var centerID string
var Config *goconfig.ConfigFile

type Index struct {
    CenterID string
    Title string
    CourseList []string
}

//上传页面
func indexHandler(w http.ResponseWriter, r *http.Request){
	uploadTemplate, err := template.ParseFiles("../templates/index.html")
	if err != nil {log.Fatal(err) }
    indexVars := Index{Title: "课程包管理", CenterID: centerID}

	path := dataDir + centerID
    os.Mkdir(path, 0777) //创建中心文件夹，如果存在则不创建
	files, err := ioutil.ReadDir(path)
	if err != nil { fmt.Fprintln(w, "Fail to read Dir:" + path, err) }

	for _,file := range files {
		if file.IsDir() {
			indexVars.CourseList = append(indexVars.CourseList, file.Name())
		} else {
			continue
		}
	}

	if err := uploadTemplate.Execute(w, indexVars); err != nil {
		log.Fatal("Execute: ", err.Error())
			return
	}
}

//上传文件
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(32 << 20)
	file, handler, err := r.FormFile("file")

	if err != nil {
		fmt.Fprintf(w, "请上传zip包") 
		return
	}

	if !strings.Contains(strings.ToLower(handler.Filename), ".zip") { 
		fmt.Fprintf(w, "请上传zip包") 
		return
	}

	defer file.Close()
//	fmt.Fprintf(w, "%v", handler.Header)
	zipFile := dataDir + centerID + "/" + handler.Filename

    os.Mkdir(dataDir + centerID, 0777) //创建中心文件夹，如果存在则不创建

	f, err := os.OpenFile(zipFile, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()
	io.Copy(f, file)

	err = unzip(zipFile, dataDir + centerID + "/")
    if err != nil { log.Fatal(err) }

	http.Redirect(w, r, "/", http.StatusFound)
}

//解压zip文件
func unzip(zipFile, dest string) error{
	r, err := zip.OpenReader(zipFile)
    if err != nil {
        return err
    }
    defer r.Close()

    for _, f := range r.File {
        rc, err := f.Open()
        if err != nil {
            return err
        }
        defer rc.Close()

        path := filepath.Join(dest, f.Name)
        if f.FileInfo().IsDir() {
            os.MkdirAll(path, f.Mode())
        } else {
            f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
            if err != nil {
                return err
            }
            defer f.Close()

            _, err = io.Copy(f, rc)
            if err != nil {
                return err
            }
        }
    }

	removeMAXOSX(dest + "__MACOSX")
	err = os.Remove(zipFile)
	if err != nil { 
		fmt.Printf("%s", err) 
		return err
	}

    return nil
}

// 删除__MACOSX目录，mac压缩的信息目录
func removeMAXOSX(dir string) {
	fileinfo, err := os.Stat(dir)
	if err != nil {return}
    if fileinfo.IsDir() { os.RemoveAll(dir) }
}

//获取课程列表, 返回json
func getCourseList(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    for k, v := range r.Form {
		if strings.ToLower(k) == "centerid" { centerID = strings.Join(v, "") }
    }

	path := dataDir + centerID
	files, err := ioutil.ReadDir(path)
	if err != nil { fmt.Fprintln(w, "Fail to read Dir:" + path, err) }

	var courseList []Course
	for _,file := range files {
		if file.IsDir() {
			courseList = append(courseList, Course{Name: file.Name()})
		} else {
			continue
		}
	}

	output, err := json.Marshal(courseList)
	if err != nil { fmt.Fprintln(w, "Json error:", err) }
	fmt.Fprintln(w, string(output))
}

//获取课程下的文件，返回json
func getCourseFiles(w http.ResponseWriter, r *http.Request) {
	courseName := ""
    r.ParseForm()
    for k, v := range r.Form {
		if strings.ToLower(k) == "centerid"   { centerID   = strings.Join(v, "") }
		if strings.ToLower(k) == "coursename" { courseName = strings.Join(v, "") }
    }
	if courseName == "" {
		fmt.Fprintln(w, "Can't get the coursename")
		return 
	}

	path := dataDir + centerID + "/" + courseName
	files, err := ioutil.ReadDir(path)
	if err != nil { fmt.Fprintln(w, "Fail to read Dir:" + path, err) }

	var courseFiles []CourseFile
	for _,file := range files {
		if file.IsDir() {
			path = path + "/" + file.Name()
			subFiles, err := ioutil.ReadDir(path)
			if err != nil { fmt.Fprintln(w, "Fail to read Dir:" + path, err) }
			Adds := []string{}
			for _,subFile := range subFiles {
				if subFile.IsDir(){
					continue
				}else{
					Adds = append(Adds, "http://" + r.Host + "/data/" + centerID + "/" + courseName + "/" + file.Name() + "/" + subFile.Name())
				}
			}
			courseFiles = append(courseFiles, CourseFile{Name: file.Name(), Add: "", Type: "folder", Adds:Adds})
		} else {
			fileType := getFileType(file.Name())
			courseFiles = append(courseFiles, CourseFile{Name: file.Name(), Add: "http://" + r.Host + "/data/" + centerID + "/" + courseName + "/" + file.Name(), Type: fileType, Adds:[]string{}})
		}
	}

	output, err := json.Marshal(courseFiles)
	if err != nil { fmt.Fprintln(w, "Json error:", err) }
	fmt.Fprintln(w, string(output))
}

//根据文件名取得文件类型, image|audio|video
func getFileType(filename string) string{
	filename = strings.ToLower(filename)

	imageTypes := "gif|jpg|jpeg|bmp|png"
	for _,extName := range strings.Split(imageTypes, "|"){
		if strings.Contains(filename, "." + extName) { return "image" }
	}

	audioTypes := "wav|mp3|wma|ape|aac"
	for _,extName := range strings.Split(audioTypes, "|"){
		if strings.Contains(filename, "." + extName) { return "audio" }
	}

	videoTypes := "wmv|avi|mp4|rmvb|3gp"
	for _,extName := range strings.Split(videoTypes, "|"){
		if strings.Contains(filename, "." + extName) { return "video" }
	}
	return "image"
}

//解析配置文件
func init() {
	Config, _ = goconfig.LoadConfigFile("../etc/config.ini")
	centerID, _ = Config.GetValue("api", "centerID")
	fmt.Println("centerID = " + centerID)
}

//删除课程包
func deleteCourse(w http.ResponseWriter, r *http.Request) {
	courseName := ""
    r.ParseForm()
    for k, v := range r.Form {
		if strings.ToLower(k) == "centerid"   { centerID   = strings.Join(v, "") }
		if strings.ToLower(k) == "coursename" { courseName = strings.Join(v, "") }
    }

	courseDir := dataDir + centerID + "/" + courseName
	err := os.RemoveAll(courseDir)
	if err != nil {fmt.Println("Fail to delete '" + courseDir + "'")}

	http.Redirect(w, r, "/", http.StatusFound)
}

func main() {
    http.HandleFunc("/", indexHandler)
    http.HandleFunc("/upload", uploadHandler)
    http.HandleFunc("/getCourseList", getCourseList)
    http.HandleFunc("/getCourseFiles", getCourseFiles)
    http.HandleFunc("/deleteCourse", deleteCourse)
	http.Handle("/data/", http.StripPrefix("/data", http.FileServer(http.Dir(dataDir))))
    http.ListenAndServe(":1234", nil)
}
