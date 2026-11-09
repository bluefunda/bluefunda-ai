package document

import (
	"net/url"
	"path/filepath"
	"strings"
)

// ParseObjectFromURI extracts objectType and objectName from a file URI.
// Convention: file:///path/to/ZPROGRAM.abap -> ("PROG", "ZPROGRAM")
// The object type is inferred from the filename prefix when possible.
func ParseObjectFromURI(uri string) (objectType, objectName string) {
	// Parse the URI to get the file path
	path := uri
	if u, err := url.Parse(uri); err == nil && u.Path != "" {
		path = u.Path
	}

	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	name = strings.ToUpper(name)

	if name == "" {
		return "", ""
	}

	// Infer type from common ABAP naming conventions
	switch {
	case strings.HasPrefix(name, "ZCL_") || strings.HasPrefix(name, "YCL_") || strings.HasPrefix(name, "CL_"):
		return "CLAS", name
	case strings.HasPrefix(name, "ZIF_") || strings.HasPrefix(name, "YIF_") || strings.HasPrefix(name, "IF_"):
		return "INTF", name
	case strings.HasPrefix(name, "ZFUGR_") || strings.HasPrefix(name, "FUGR_"):
		return "FUGR", name
	default:
		// Default to program for Z/Y prefixed or anything else
		return "PROG", name
	}
}

// ObjectTypeToFileExt returns the conventional file extension for an ABAP object type
func ObjectTypeToFileExt(objectType string) string {
	switch strings.ToUpper(objectType) {
	case "CLAS", "CLASS":
		return ".clas.abap"
	case "INTF", "INTERFACE":
		return ".intf.abap"
	case "FUGR", "FUNCTION_GROUP":
		return ".fugr.abap"
	default:
		return ".abap"
	}
}
