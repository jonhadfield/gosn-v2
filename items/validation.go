package items

import (
	"fmt"
	"log"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func validateContentSchema(schema *jsonschema.Schema, itemContent interface{}) error {
	if schema == nil {
		return fmt.Errorf("schema is nil")
	}

	if itemContent == nil {
		log.Print("itemContent is nil")
		return nil
	}

	if err := schema.Validate(itemContent); err != nil {
		return err
	}

	return nil
}
