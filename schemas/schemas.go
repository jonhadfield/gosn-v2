package schemas

import (
	"embed"
	"fmt"
	"io/fs"
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
		return nil, err
	}

	for _, e := range rSchemas {
		var sB []byte

		// even windows uses forward slashes when references embedded files
		sB, err = fs.ReadFile(fsSchemas, fmt.Sprintf("%s/%s", embedFilesDirName, e.Name()))
		if err != nil {
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
