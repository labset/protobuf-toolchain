package codegen

import (
	"embed"
	"fmt"
	"path"
	"strings"
	"text/template"

	pluginV1 "github.com/labset/protobuf-toolchain/api/labset/plugin/v1"
	"github.com/labset/protobuf-toolchain/internal/entity"
	"github.com/labset/protobuf-toolchain/internal/helpers"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
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
	// schema overrides the Postgres schema the tables live under; empty derives
	// it per package from the proto package name.
	schema string
}

func (g *goSqlcAtlasGenerator) Generate(plugin *protogen.Plugin) error {
	tmpl, err := parseTemplates()
	if err != nil {
		return err
	}

	dirs, order, err := g.collectDirs(plugin)
	if err != nil {
		return err
	}

	for _, dir := range order {
		if err = renderDir(plugin, tmpl, dir, dirs[dir]); err != nil {
			return err
		}
	}

	return nil
}

func parseTemplates() (*template.Template, error) {
	return template.New("").
		Funcs(template.FuncMap{"has": hasOperation}).
		ParseFS(goSqlcAtlasTemplateFS, "templates/go-sqlc-atlas/*.tmpl")
}

func hasOperation(ops []string, op string) bool {
	for _, o := range ops {
		if o == op {
			return true
		}
	}
	return false
}

// collectDirs groups every ROLE_ENTITY message by output directory; order keeps
// the directories in first-seen order for deterministic output.
func (g *goSqlcAtlasGenerator) collectDirs(
	plugin *protogen.Plugin,
) (map[string]*dirModel, []string, error) {
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
			opts := entity.MessageAnnotation(message.Desc)
			if opts == nil || opts.GetRole() != pluginV1.Role_ROLE_ENTITY {
				continue
			}

			dm, created, err := g.dirModelFor(dirs, dir, file)
			if err != nil {
				return nil, nil, err
			}
			if created {
				order = append(order, dir)
			}
			dm.Entities = append(dm.Entities, buildEntity(message, opts))
		}
	}

	return dirs, order, nil
}

// dirModelFor returns the directory's model, creating it (created=true) on first
// sight. Two proto packages in one directory is an error: the per-directory
// config cannot represent both.
func (g *goSqlcAtlasGenerator) dirModelFor(
	dirs map[string]*dirModel,
	dir string,
	file *protogen.File,
) (*dirModel, bool, error) {
	pkg := string(file.Desc.Package())
	if dm := dirs[dir]; dm != nil {
		if dm.Package != pkg {
			return nil, false, fmt.Errorf(
				"go-sqlc-atlas: directory %q holds two proto packages %q and %q",
				dir, dm.Package, pkg,
			)
		}
		return dm, false, nil
	}

	dm := &dirModel{
		Source:       dir,
		Package:      pkg,
		GoPackage:    string(file.GoPackageName),
		Schema:       g.schemaFor(file.Desc.Package()),
		Migration:    g.migration,
		goImportPath: file.GoImportPath,
	}
	dirs[dir] = dm
	return dm, true, nil
}

// schemaFor returns the Postgres schema a directory's tables live under: the
// explicit override when set, otherwise the proto package with dots replaced by
// underscores (projectmanagement.v1 -> projectmanagement_v1). A package-less
// file falls back to "public" so the emitted SQL stays valid.
func (g *goSqlcAtlasGenerator) schemaFor(pkg protoreflect.FullName) string {
	if g.schema != "" {
		return g.schema
	}
	if pkg == "" {
		return "public"
	}
	return strings.ReplaceAll(string(pkg), ".", "_")
}

// renderDir emits one output layer for a directory: sql/schema.sql (all
// entities), sql/queries/<table>.sql (one per entity), and the sqlc/atlas
// config plus generate.go at the directory root.
func renderDir(
	plugin *protogen.Plugin,
	tmpl *template.Template,
	dir string,
	dm *dirModel,
) error {
	schema := plugin.NewGeneratedFile(path.Join(dir, "sql", "schema.sql"), dm.goImportPath)
	if err := tmpl.ExecuteTemplate(schema, "schema.sql.tmpl", dm); err != nil {
		return err
	}

	seen := make(map[string]string)
	for _, ent := range dm.Entities {
		filename := path.Join(dir, "sql", "queries", ent.Table+".sql")
		if prev, ok := seen[filename]; ok {
			return fmt.Errorf(
				"go-sqlc-atlas: entities %q and %q both generate %q",
				prev, ent.Entity, filename,
			)
		}
		seen[filename] = ent.Entity

		out := plugin.NewGeneratedFile(filename, dm.goImportPath)
		if err := tmpl.ExecuteTemplate(out, "query.sql.tmpl",
			queryFileModel{Source: dm.Source, Schema: dm.Schema, sqlEntity: ent}); err != nil {
			return err
		}
	}

	configs := []struct{ filename, template string }{
		{"sqlc.yaml", "sqlc.yaml.tmpl"},
		{"atlas.hcl", "atlas.hcl.tmpl"},
		{"generate.go", "generate.go.tmpl"},
	}
	for _, c := range configs {
		out := plugin.NewGeneratedFile(path.Join(dir, c.filename), dm.goImportPath)
		if err := tmpl.ExecuteTemplate(out, c.template, dm); err != nil {
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
	Schema    string // Postgres schema the tables live under
	Migration string // "" (declarative) or an Atlas migration format
	Entities  []sqlEntity

	goImportPath protogen.GoImportPath
}

// queryFileModel is the input for query.sql.tmpl: one entity's queries plus the
// schema they are qualified with and the directory recorded in the file header.
type queryFileModel struct {
	Source string
	Schema string
	sqlEntity
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

func buildEntity(message *protogen.Message, opts *pluginV1.MessageOptions) sqlEntity {
	name := string(message.Desc.Name())
	ent := sqlEntity{
		Entity: name,
		Table:  helpers.ToSnake(name),
	}

	for _, field := range message.Fields {
		if col, ok := columnFor(field); ok {
			ent.Columns = append(ent.Columns, col)
		}
	}

	for _, op := range entity.DistinctOperations(opts) {
		ent.Operations = append(ent.Operations, entity.OperationName(op))
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

// columnFor maps a message field to a Postgres column, or reports false when the
// field does not become one.
func columnFor(field *protogen.Field) (sqlColumn, bool) {
	if field.Desc.IsList() || field.Desc.IsMap() {
		return sqlColumn{}, false
	}

	// The Entity base's columns are emitted by the templates, not derived here.
	if field.Message != nil && string(field.Message.Desc.FullName()) == entity.BaseFullName {
		return sqlColumn{}, false
	}

	name := helpers.ToSnake(string(field.Desc.Name()))
	if reservedColumns[name] {
		return sqlColumn{}, false
	}

	// A field carries a NULL-able column when it has explicit presence: proto3
	// optional, a message type, or a member of a (real or synthetic) oneof.
	nullable := field.Desc.HasPresence()

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
		// A non-Timestamp message is a foreign key, deferred for now.
		if field.Message == nil ||
			string(field.Message.Desc.FullName()) != "google.protobuf.Timestamp" {
			return sqlColumn{}, false
		}
		sqlType = "timestamptz"
	default:
		return sqlColumn{}, false
	}

	return sqlColumn{Name: name, SQLType: sqlType, Nullable: nullable}, true
}
