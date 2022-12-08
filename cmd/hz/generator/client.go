package generator

import (
	"path/filepath"

	"github.com/cloudwego/hertz/cmd/hz/generator/model"
	"github.com/cloudwego/hertz/cmd/hz/util"
)

type ClientMethod struct {
	*HttpMethod
	BodyParamsCode   string
	QueryParamsCode  string
	PathParamsCode   string
	HeaderParamsCode string
	FormValueCode    string
	FormFileCode     string
}

type ClientFile struct {
	FilePath      string
	ServiceName   string
	Imports       map[string]*model.Model
	ClientMethods []*ClientMethod
}

func (pkgGen *HttpPackageGenerator) genClient(pkg *HttpPackage, clientDir string) error {
	for _, s := range pkg.Services {
		hertzClientPath := filepath.Join(clientDir, "hertz_client.go")
		isExist, err := util.PathExist(hertzClientPath)
		if err != nil {
			return err
		}
		if !isExist {
			err := pkgGen.TemplateGenerator.Generate(nil, hertzClientTplName, hertzClientPath, false)
			if err != nil {
				return err
			}
		}
		var client ClientFile
		client = ClientFile{
			FilePath:      filepath.Join(clientDir, util.ToSnakeCase(s.Name)+".go"),
			ServiceName:   util.ToSnakeCase(s.Name),
			ClientMethods: s.ClientMethods,
		}
		client.Imports = make(map[string]*model.Model, len(client.ClientMethods))
		for _, m := range client.ClientMethods {
			// Iterate over the request and return parameters of the method to get import path.
			for key, mm := range m.Models {
				if v, ok := client.Imports[mm.PackageName]; ok && v.Package != mm.Package {
					client.Imports[key] = mm
					continue
				}
				client.Imports[mm.PackageName] = mm
			}
		}
		err = pkgGen.TemplateGenerator.Generate(client, serviceClientName, client.FilePath, false)
		if err != nil {
			return err
		}
	}
	return nil
}
