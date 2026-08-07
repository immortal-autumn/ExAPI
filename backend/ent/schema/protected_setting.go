package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// ProtectedSetting stores explicitly classified setting values outside the
// legacy plaintext settings table. Envelope may be either a data-encryption
// envelope or a keyed digest according to the application registry.
type ProtectedSetting struct {
	ent.Schema
}

func (ProtectedSetting) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "protected_settings"}}
}

func (ProtectedSetting) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").MaxLen(100).NotEmpty().Unique(),
		field.String("envelope").SchemaType(map[string]string{dialect.Postgres: "text"}).Sensitive(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}
