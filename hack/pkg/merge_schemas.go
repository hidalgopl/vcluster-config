package pkg

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/loft-sh/vcluster-config/config"

	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

const (
	externalConfigName = "ExternalConfig"
	platformConfigName = "PlatformConfig"
	platformConfigRef  = "#/defs/" + platformConfigName
	externalConfigRef  = "#/defs/" + externalConfigName
)

func RunMergeSchemas(valuesSchemaFile, platformConfigSchemaFile, outFile string) error {
	platformBytes, err := os.ReadFile(platformConfigSchemaFile)
	if err != nil {
		return err
	}
	platformSchema := &jsonschema.Schema{}
	err = json.Unmarshal(platformBytes, platformSchema)
	if err != nil {
		return err
	}

	valuesBytes, err := os.ReadFile(valuesSchemaFile)
	if err != nil {
		return err
	}
	valuesSchema := &jsonschema.Schema{}
	err = json.Unmarshal(valuesBytes, valuesSchema)
	if err != nil {
		return err
	}

	if err := addPlatformSchema(platformSchema, valuesSchema); err != nil {
		return err
	}

	return writeSchema(valuesSchema, outFile)
}

func addPrefixToCommentsKey(commentsMap map[string]string) map[string]string {
	result := make(map[string]string, len(commentsMap))
	for k, v := range commentsMap {
		result["github.com/loft-sh/vcluster-config/"+k] = v
	}
	return result
}

func addPlatformSchema(platformSchema, toSchema *jsonschema.Schema) error {
	commentsMap := make(map[string]string)
	r := new(jsonschema.Reflector)
	r.RequiredFromJSONSchemaTags = true
	r.BaseSchemaID = "https://vcluster.com/schemas"
	r.ExpandedStruct = true

	if err := jsonschema.ExtractGoComments("./", "config", commentsMap); err != nil {
		return err
	}
	r.CommentMap = addPrefixToCommentsKey(commentsMap)
	platformConfigSchema := r.Reflect(&config.PlatformConfig{})
	if err := writeSchema(platformConfigSchema, "here.platform.schema.json"); err != nil {
		return err
	}
	platformNode := &jsonschema.Schema{
		AdditionalProperties: nil,
		Description:          platformConfigName + " holds platform configuration",
		Properties:           platformSchema.Properties,
		Type:                 "object",
	}
	for pair := platformConfigSchema.Properties.Oldest(); pair != nil; pair = pair.Next() {
		platformNode.Properties.AddPairs(
			orderedmap.Pair[string, *jsonschema.Schema]{
				Key:   pair.Key,
				Value: pair.Value,
			})
	}

	for k, v := range platformConfigSchema.Definitions {
		if k == "PlatformConfig" {
			continue
		}
		toSchema.Definitions[k] = v
	}

	for pair := platformConfigSchema.Properties.Oldest(); pair != nil; pair = pair.Next() {
		pair := pair
		platformNode.Properties.AddPairs(*pair)
	}

	toSchema.Definitions[platformConfigName] = platformNode
	properties := jsonschema.NewProperties()
	properties.AddPairs(orderedmap.Pair[string, *jsonschema.Schema]{
		Key: "platform",
		Value: &jsonschema.Schema{
			Ref:         platformConfigRef,
			Description: "platform holds platform configuration",
			Type:        "object",
		},
	})
	externalConfigNode, ok := toSchema.Definitions[externalConfigName]
	if !ok {
		externalConfigNode = &jsonschema.Schema{
			AdditionalProperties: nil,
			Description:          externalConfigName + " holds external configuration",
			Properties:           properties,
			Ref:                  externalConfigRef,
		}
	} else {
		externalConfigNode.Properties = properties
		externalConfigNode.Description = externalConfigName + " holds external configuration"
		externalConfigNode.Ref = externalConfigRef
	}
	toSchema.Definitions[externalConfigName] = externalConfigNode

	for defName, node := range platformSchema.Definitions {
		if _, exists := toSchema.Definitions[defName]; exists {
			panic("trying to overwrite definition " + defName + " this is unexpected")
		}
		toSchema.Definitions[defName] = node
	}
	if externalProperty, ok := toSchema.Properties.Get("external"); !ok {
		return nil
	} else {
		externalProperty.Ref = externalConfigRef
		externalProperty.AdditionalProperties = nil
		externalProperty.Type = ""
	}
	return nil
}

func writeSchema(schema *jsonschema.Schema, schemaFile string) error {
	prefix := ""
	schemaString, err := json.MarshalIndent(schema, prefix, "  ")
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(schemaFile), os.ModePerm)
	if err != nil {
		return err
	}
	if _, err = os.Create(schemaFile); err != nil {
		return err
	}

	err = os.WriteFile(schemaFile, schemaString, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}
