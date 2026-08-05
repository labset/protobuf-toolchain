package codegen

import (
	"embed"
	"path"
	"strings"
	"text/template"

	pluginV1 "github.com/labset/protobuf-toolchain/api/labset/plugin/v1"
	"github.com/labset/protobuf-toolchain/internal/helpers"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

//go:embed templates/go-sqlc-atlas/*.tmpl
var goSqlcAtlasTemplateFS embed.FS

// goSqlcAtlasGenerator turns every ROLE_ENTITY message into a Postgres backend
// layer: a CREATE TABLE schema, sqlc CRUD queries, and the sqlc/atlas config
// plus a generate.go that drives them. Entities sharing an output directory are
// aggregated into one schema.sql / query.sql and one config set.
type goSqlcAtlasGenerator struct {
	// migration is the Atlas migration directory format (goose, flyway, ...);
	// empty selects declarative schema management (atlas schema apply).
	migration string
}

func (g *goSqlcAtlasGenerator) Generate(plugin *protogen.Plugin) error {
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"has": func(ops []string, op string) bool {
			for _, o := range ops {
				if o == op {
					return true
				}
			}
			return false
		},
	}).ParseFS(goSqlcAtlasTemplateFS, "templates/go-sqlc-atlas/*.tmpl")
	if err != nil {
		return err
	}

	// Entities are grouped by output directory: schema.sql, query.sql and the
	// config files are per-directory artifacts, so every entity in a directory
	// contributes to one shared set. order preserves first-seen directory order
	// for deterministic output.
	dirs := make(map[string]*dirModel)
	var order []string

	for _, file := range plugin.Files {
		if !file.Generate {
			continue
		}
		dir := path.Dir(file.Desc.Path())

		// Only top-level messages model first-class tables; a nested entity
		// annotation is a lint concern, skipped here (see proto-service).
		for _, message := range file.Messages {
			opts := entityMessageOptions(message)
			if opts == nil || opts.GetRole() != pluginV1.Role_ROLE_ENTITY {
				continue
			}

			dm := dirs[dir]
			if dm == nil {
				dm = &dirModel{
					Source:       dir,
					Package:      string(file.Desc.Package()),
					GoPackage:    string(file.GoPackageName),
					Migration:    g.migration,
					goImportPath: file.GoImportPath,
				}
				dirs[dir] = dm
				order = append(order, dir)
			}
			dm.Entities = append(dm.Entities, buildEntity(message, opts))
		}
	}

	for _, dir := range order {
		if err = renderDir(plugin, tmpl, dir, dirs[dir]); err != nil {
			return err
		}
	}

	return nil
}

// renderDir emits the five per-directory artifacts for one output directory.
func renderDir(
	plugin *protogen.Plugin,
	tmpl *template.Template,
	dir string,
	dm *dirModel,
) error {
	artifacts := []struct{ filename, template string }{
		{"schema.sql", "schema.sql.tmpl"},
		{"query.sql", "query.sql.tmpl"},
		{"sqlc.yaml", "sqlc.yaml.tmpl"},
		{"atlas.hcl", "atlas.hcl.tmpl"},
		{"generate.go", "generate.go.tmpl"},
	}

	for _, a := range artifacts {
		out := plugin.NewGeneratedFile(path.Join(dir, a.filename), dm.goImportPath)
		if err := tmpl.ExecuteTemplate(out, a.template, dm); err != nil {
			return err
		}
	}

	return nil
}

// dirModel is the input for every go-sqlc-atlas template.
type dirModel struct {
	Source    string // output directory, recorded in file headers
	Package   string // proto package, e.g. projectmanagement.v1
	GoPackage string // Go package name for generate.go
	Migration string // "" (declarative) or an Atlas migration format
	Entities  []sqlEntity

	goImportPath protogen.GoImportPath
}

// sqlEntity is a single table and its CRUD queries.
type sqlEntity struct {
	Entity     string      // Project
	Table      string      // project
	Columns    []sqlColumn // business columns (excludes the Entity base columns)
	Operations []string    // CREATE | READ | UPDATE | DELETE | LIST
}

// sqlColumn is one business column of a table.
type sqlColumn struct {
	Name     string // snake_case column name
	SQLType  string // text | bigint | boolean | timestamptz | ...
	Nullable bool
}

// buildEntity derives the table and query model for a ROLE_ENTITY message. The
// embedded labset.plugin.v1.Entity field contributes the fixed base columns
// (id, created_at, updated_at, deleted_at) owned by the templates, so it — and
// any field colliding with those reserved names — is excluded here.
func buildEntity(message *protogen.Message, opts *pluginV1.MessageOptions) sqlEntity {
	entity := string(message.Desc.Name())
	ent := sqlEntity{
		Entity: entity,
		Table:  helpers.ToSnake(entity),
	}

	for _, field := range message.Fields {
		if col, ok := columnFor(field); ok {
			ent.Columns = append(ent.Columns, col)
		}
	}

	seen := make(map[pluginV1.Operation]bool)
	for _, op := range opts.GetOperations() {
		if op == pluginV1.Operation_OPERATION_UNSPECIFIED || seen[op] {
			continue
		}
		seen[op] = true
		ent.Operations = append(ent.Operations, strings.TrimPrefix(op.String(), "OPERATION_"))
	}

	return ent
}

// reservedColumns are owned by the embedded Entity base and emitted by the
// templates; a business field of the same name would duplicate them.
var reservedColumns = map[string]bool{
	"id":         true,
	"created_at": true,
	"updated_at": true,
	"deleted_at": true,
}

// columnFor maps a message field to a Postgres column, reporting false for
// fields that do not become columns: the embedded Entity base, reserved names,
// repeated fields, and message types other than Timestamp (foreign keys are a
// deferred story). Unmapped kinds are skipped rather than failing the run.
func columnFor(field *protogen.Field) (sqlColumn, bool) {
	if field.Desc.IsList() || field.Desc.IsMap() {
		return sqlColumn{}, false
	}

	name := helpers.ToSnake(string(field.Desc.Name()))
	if reservedColumns[name] {
		return sqlColumn{}, false
	}

	nullable := field.Desc.HasOptionalKeyword()

	var sqlType string
	switch field.Desc.Kind() {
	case protoreflect.StringKind:
		sqlType = "text"
	case protoreflect.BoolKind:
		sqlType = "boolean"
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		sqlType = "integer"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		sqlType = "bigint"
	case protoreflect.FloatKind:
		sqlType = "real"
	case protoreflect.DoubleKind:
		sqlType = "double precision"
	case protoreflect.MessageKind:
		if field.Message == nil ||
			string(field.Message.Desc.FullName()) != "google.protobuf.Timestamp" {
			return sqlColumn{}, false
		}
		sqlType = "timestamptz"
		nullable = true
	default:
		return sqlColumn{}, false
	}

	return sqlColumn{Name: name, SQLType: sqlType, Nullable: nullable}, true
}

// entityMessageOptions returns the labset.plugin.v1.MessageOptions annotation on
// a message, or nil when it carries none.
func entityMessageOptions(message *protogen.Message) *pluginV1.MessageOptions {
	opts, ok := message.Desc.Options().(*descriptorpb.MessageOptions)
	if !ok || opts == nil {
		return nil
	}
	if !proto.HasExtension(opts, pluginV1.E_Message) {
		return nil
	}

	msgOpts, _ := proto.GetExtension(opts, pluginV1.E_Message).(*pluginV1.MessageOptions)
	return msgOpts
}
