// Package mongodb is the shared MongoDB setup for services that own a Mongo database.
package mongodb

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Config is embedded by services with an envPrefix, e.g. `envPrefix:"CATALOG_MONGO_"`.
type Config struct {
	URI  string `env:"URI,required"`
	Name string `env:"NAME,required"`
}

// Connect returns the service's database, failing fast if the server is unreachable.
func Connect(ctx context.Context, cfg Config) (*mongo.Database, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(cfg.URI).SetRegistry(newRegistry()))
	if err != nil {
		return nil, fmt.Errorf("connecting to mongodb: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("pinging mongodb: %w", err)
	}

	return client.Database(cfg.Name), nil
}

var uuidType = reflect.TypeFor[uuid.UUID]()

// UUIDs are stored as canonical strings so ids match the other services and read cleanly in mongosh.
func newRegistry() *bson.Registry {
	reg := bson.NewRegistry()
	reg.RegisterTypeEncoder(uuidType, bson.ValueEncoderFunc(encodeUUID))
	reg.RegisterTypeDecoder(uuidType, bson.ValueDecoderFunc(decodeUUID))
	return reg
}

func encodeUUID(_ bson.EncodeContext, vw bson.ValueWriter, val reflect.Value) error {
	id, ok := val.Interface().(uuid.UUID)
	if !ok {
		return fmt.Errorf("encoding uuid: unexpected type %s", val.Type())
	}
	return vw.WriteString(id.String())
}

func decodeUUID(_ bson.DecodeContext, vr bson.ValueReader, val reflect.Value) error {
	s, err := vr.ReadString()
	if err != nil {
		return fmt.Errorf("decoding uuid: %w", err)
	}

	id, err := uuid.Parse(s)
	if err != nil {
		return fmt.Errorf("decoding uuid: %w", err)
	}

	val.Set(reflect.ValueOf(id))
	return nil
}
