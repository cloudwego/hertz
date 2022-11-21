/*
 * Copyright 2022 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package generator

import (
	"testing"
	"text/template"

	"github.com/cloudwego/hertz/cmd/hz/generator/model"
	"github.com/cloudwego/hertz/cmd/hz/meta"
)

type StringValue struct {
	src string
}

func (sv *StringValue) Expression() string {
	return sv.src
}

func TestIdlGenerator_GenModel(t *testing.T) {
	typeModel := &model.Type{
		Name:     "Model",
		Kind:     model.KindStruct,
		Indirect: true,
	}
	typeErr := &model.Type{
		Name:     "error",
		Kind:     model.KindInterface,
		Indirect: false,
	}

	type fields struct {
		ConfigPath  string
		OutputDir   string
		Backend     meta.Backend
		handlerDir  string
		routerDir   string
		modelDir    string
		ProjPackage string
		Config      *TemplateConfig
		tpls        map[string]*template.Template
	}
	type args struct {
		data *model.Model
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "",
			fields: fields{
				OutputDir: "./testdata",
				Backend:   meta.BackendGolang,
			},
			args: args{
				data: &model.Model{
					FilePath:    "idl/main.thrift",
					Package:     "model/psm",
					PackageName: "psm",
					Imports: map[string]*model.Model{
						"base": {
							Package:     "model/base",
							PackageName: "base",
						},
					},
					Typedefs: []model.TypeDef{
						{
							Alias: "HerztModel",
							Type:  typeModel,
						},
					},
					Constants: []model.Constant{
						{
							Name:  "OBJ",
							Type:  typeErr,
							Value: &StringValue{"fmt.Errorf(\"EOF\")"},
						},
					},
					Variables: []model.Variable{
						{
							Name:  "Object",
							Type:  typeModel,
							Value: &StringValue{"&Model{}"},
						},
					},
					Functions: []model.Function{
						{
							Name: "Init",
							Args: nil,
							Rets: []model.Variable{
								{
									Name: "err",
									Type: typeErr,
								},
							},
							Code: "return nil",
						},
					},
					Enums: []model.Enum{
						{
							Name: "Sex",
							Values: []model.Constant{
								{
									Name: "Male",
									Type: &model.Type{
										Name:     "int",
										Kind:     model.KindInt,
										Indirect: false,
										Category: 1,
									},
									Value: &StringValue{"1"},
								},
								{
									Name: "Femal",
									Type: &model.Type{
										Name:     "int",
										Kind:     model.KindInt,
										Indirect: false,
										Category: 1,
									},
									Value: &StringValue{"2"},
								},
							},
						},
					},
					Structs: []model.Struct{
						{
							Name: "Model",
							Fields: []model.Field{
								{
									Name: "A",
									Type: &model.Type{
										Name:     "[]byte",
										Kind:     model.KindSlice,
										Indirect: false,
										Category: model.CategoryBinary,
									},
									IsSetDefault: true,
									DefaultValue: &StringValue{"[]byte(\"\")"},
								},
								{
									Name: "B",
									Type: &model.Type{
										Name:     "Base",
										Kind:     model.KindStruct,
										Indirect: false,
									},
								},
							},
							Category: model.CategoryUnion,
						},
					},
					Methods: []model.Method{
						{
							ReceiverName: "self",
							ReceiverType: typeModel,
							ByPtr:        true,
							Function: model.Function{
								Name: "Bind",
								Args: []model.Variable{
									{
										Name: "c",
										Type: &model.Type{
											Name: "RequestContext",
											Scope: &model.Model{
												PackageName: "hertz",
											},
											Kind:     model.KindStruct,
											Indirect: true,
										},
									},
								},
								Rets: []model.Variable{
									{
										Name: "error",
										Type: typeErr,
									},
								},
								Code: "return nil",
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			self := &HttpPackageGenerator{
				ConfigPath:  tt.fields.ConfigPath,
				Backend:     tt.fields.Backend,
				HandlerDir:  tt.fields.handlerDir,
				RouterDir:   tt.fields.routerDir,
				ModelDir:    tt.fields.modelDir,
				ProjPackage: tt.fields.ProjPackage,
				TemplateGenerator: TemplateGenerator{
					OutputDir: tt.fields.OutputDir,
					Config:    tt.fields.Config,
					tpls:      tt.fields.tpls,
				},
				Options: []Option{
					OptionTypedefAsTypeAlias,
					OptionMarshalEnumToText,
				},
			}

			err := self.LoadBackend(meta.BackendGolang)
			if err != nil {
				t.Fatal(err)
			}

			if err := self.GenModel(tt.args.data, true); (err != nil) != tt.wantErr {
				t.Errorf("IdlGenerator.GenModel() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := self.Persist(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
