package schemas

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed files/*
var fsSchemas embed.FS

const embedFilesDirName = "files"

func LoadSchemas() (map[string]*jsonschema.Schema, error) {
	cSchemas := make(map[string]*jsonschema.Schema)

	rSchemas, err := fsSchemas.ReadDir(embedFilesDirName)
	if err != nil {
		fmt.Printf("error reading schemas: %s\n", err)
		return nil, err
	}

	for _, e := range rSchemas {
		var sB []byte

		var cwd string
		cwd, err = os.Getwd()
		if err != nil {
			return nil, err
		}

		fmt.Printf("cwd: %s\n", cwd)
		fmt.Println("e.Name(): ", e.Name())

		var absPath string
		absPath, err = filepath.Abs(cwd)
		if err != nil {
			return nil, err
		}
		fmt.Println("abs path:", absPath)

		// fmt.Println("going to reach schema file from:", filepath.Join(absPath, embedFilesDirName, e.Name()))
		fmt.Println("going to reach schema file from: files/note.json")

		// sB, err = fs.ReadFile(fsSchemas, filepath.Join(absPath, embedFilesDirName, e.Name()))
		sB, err = fs.ReadFile(fsSchemas, fmt.Sprintf("%s/%s", embedFilesDirName, e.Name()))
		if err != nil {
			fmt.Printf("error reading schemas0: %s\n", err)

			return nil, err
		}

		sName := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		sName = strings.ReplaceAll(sName, "|", "-")

		cSchemas[sName], err = jsonschema.CompileString(e.Name(), string(sB))
		if err != nil {
			return nil, err
		}
	}

	return cSchemas, nil
}
