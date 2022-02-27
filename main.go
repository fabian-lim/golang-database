package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/jcelliott/lumber"
)

const Version = "1.0.1"

type (
	Logger interface {
		Fatal(string, ...interface{})
		Error(string, ...interface{})
		Warning(string, ...interface{})
		Info(string, ...interface{})
		Debug(string, ...interface{})
		Trace(string, ...interface{})
	}

	Driver struct{
		mutex sync.Mutex // to write and delete
		mutexes map[string]*sync.Mutex // a pointer to sync.Mutex
		dir string
		log Logger
	}
)

type Options struct {
	Logger
}

//These are Struct methods, not exactly functions
func New(dir string, options *Options)(*Driver, error){ // initialise and create new database, returns Driver or error
	dir = filepath.Clean(dir)

	opts := Options{} //empty right now
	
	if options != nil {
		opts = *options 
	}
	if opts.Logger == nil {
		opts.Logger = lumber.NewConsoleLogger((lumber.INFO))
	}
	driver := Driver{
		dir: dir,
		mutexes: make(map[string]*sync.Mutex),
		log: opts.Logger,
	}
	// check if the database exist, if it does then we just use the directory
	if _,err := os.Stat(dir); err == nil{
		opts.Logger.Debug("Using '%s' (database already exists)\n", dir)
		return  &driver, nil
	}

	opts.Logger.Debug("Creating the database at '%s'...\n", dir)
	return &driver, os.Mkdir(dir, 0755) //0755 is the access permission
}

func (d *Driver) Write(collection, resource string, v interface{}) error { //retuns error only
	if collection == ""{
		return fmt.Errorf("Missing collection - no place to save record!")
	}

	if resource == ""{
		return fmt.Errorf("Missing resource - unable to save record (no name)!")
	}

	mutex := d.GetOrCreateMutex(collection)
	mutex.Lock()
	
	// defer is used when you want something to run at the end of the function
	// everything is locked until the right function is completed, otherwise it wont allow anything to work with the db
	defer mutex.Unlock()

	dir := filepath.Join(d.dir, collection)
	fnlPath := filepath.Join(dir, resource + ".json")
	tmpPath := fnlPath + ".tmp"

	if err := os.MkdirAll(dir, 0755); err != nil{
		return err
	}

	// converting 
	b, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return err
	}

	b = append(b, byte('\n'))

	if err := ioutil.WriteFile(tmpPath, b, 0644); err != nil {
		return err
	}

	return os.Rename(tmpPath, fnlPath)
}

func (d *Driver) Read(collection, resource string, v interface{}) error {
	if collection == ""{
		return fmt.Errorf("Missing collection - unable to read!")
	}

	if resource == ""{
		return fmt.Errorf("Missing resource - unable to read record!")
	}

	record := filepath.Join(d.dir, collection, resource)

	if _, err := stat(record); err != nil{
		return err
	}

	b, err := ioutil.ReadFile(record + ".json")
	if err != nil {
		return err
	}

	return json.Unmarshal(b, &v)
}

func (d *Driver) ReadAll(collection string)([]string, error){
	
	if collection == ""{
		return nil, fmt.Errorf("Missing collection - unable to read")
	}
	dir := filepath.Join(d.dir, collection)

	// checks if the collection or directory exists
	if _, err := stat(dir); err != nil {
		return nil, err
	}

	files, _ := ioutil.ReadDir(dir)

	var records []string

	for _, file := range files{
		b, err := ioutil.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return nil, err
		}
		records = append(records, string(b))
	}

	return records, nil
}

func (d *Driver) Delete(collection, resource string)error{
	
	path := filepath.Join(collection, resource)
	mutex := d.GetOrCreateMutex(collection)
	mutex.Lock()
	defer mutex.Unlock()

	dir := filepath.Join(d.dir, path)

	switch fi, err := stat(dir); {
	case fi == nil, err != nil:
		return fmt.Errorf("unable to find file or directory named %v\n", path)
	
	case fi.Mode().IsDir():
		return os.RemoveAll(dir)
		
	case fi.Mode().IsRegular():
		return os.RemoveAll(dir + ".json") //removing all the files in the folder
	}
	
	return nil
}

func (d *Driver) GetOrCreateMutex(collection string) *sync.Mutex{ //returns pointer to sync.mutex
	
	d.mutex.Lock()
	defer d.mutex.Unlock()
	m, ok := d.mutexes[collection]

	if !ok {
		m = &sync.Mutex{}
		d.mutexes[collection] = m
	}
	return m
}

// checks for the file with json
func stat(path string)(fi os.FileInfo, err error){
	if fi, err = os.Stat(path); os.IsNotExist(err){
		fi, err = os.Stat(path + ".json") // the database that we create will have a .json at the end
	}
	return 
}

type Address struct {
	City string
	State string
	Country string
	Postcode json.Number
}

type User struct {
	Name string
	Age json.Number
	Contact string
	Company string
	Address Address
}

func main() {
	dir := "./"

	db, err := New(dir, nil)
	if err != nil {
		fmt.Print("Error: ", err)
	}

	employees := []User {
		{"John", "23", "0123456789", "Google", Address{"New York", "New York", "America", "58200"}},
		{"Jim", "29", "0168426982", "Dunder Mifflin", Address{"Scranton", "Pennsylvania", "America", "42200"}},
		{"Dwight", "35", "0172698324", "Microsoft", Address{"New York City", "New York", "America", "68000"}},
		{"Michael", "40", "0136524856", "Netflix", Address{"Silicon Valley", "California", "America", "57100"}},
		{"Pam", "28", "0123652486", "Amazon", Address{"Brooklyn", "New York", "America", "52100"}},
	}

	for _, value := range employees {
		db.Write("users", value.Name, User{
			Name: value.Name,
			Age: value.Age,
			Contact: value.Contact,
			Company: value.Company,
			Address: value.Address, 
		})
	}

	records, err := db.ReadAll("users")
	if err != nil {
		fmt.Println("Error: ", err)
	}
	fmt.Println(records)

	allusers := []User{}

	for _, f := range records{
		employeeFound := User{}
		if err := json.Unmarshal([]byte(f), &employeeFound); err != nil{
			fmt.Println("Error: ", err)
		}
		allusers = append(allusers, employeeFound)
	}
	fmt.Println((allusers))

	// if err := db.Delete("user", "john"); err!= nil {
	// 	fmt.Println("Error: ", err)
	// }
	// if err := db.Delete("user", ""); err != nil {
	// 	fmt.Println("Error: ", err)
	// }
}