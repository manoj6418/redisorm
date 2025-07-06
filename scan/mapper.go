package scan

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/redis/go-redis/v9"
)

// Public Helper Functions
// Decode decodes an FT.SEARCH reply into a single T.
// T can be a struct (tagged with `redisorm:"@field"`) or map[string]string.
// If T is a struct, it must have fields tagged with `redisorm:"@field"`
// to map Redisearch fields to struct fields.

// DecodeSlice decodes an FT.SEARCH reply into []T.
// T can be a struct (tagged with `redisorm:"@field"`) or map[string]string.
func DecodeSlice[T any](raw any) ([]T, error) {
	reply, err := normalize(raw)
	if err != nil {
		return nil, err
	}
	total, hits, err := extractHits(reply)
	if err != nil {
		return nil, err
	}

	out := make([]T, total)
	for i, kv := range hits {
		m, err := toStrMap(kv)
		if err != nil {
			return nil, err
		}
		if err := assign(&out[i], m); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// DecodeMaps decodes an FT.AGGREGATE reply into []map[string]string.
func DecodeMaps(raw any) ([]map[string]string, error) {
	reply, err := normalize(raw)
	if err != nil {
		return nil, err
	}
	total, hits, err := extractHits(reply)
	if err != nil {
		return nil, err
	}

	out := make([]map[string]string, total)
	for i, kv := range hits {
		m, err := toStrMap(kv)
		if err != nil {
			return nil, err
		}
		out[i] = m
	}
	return out, nil
}

/*───────────────────────────────
|  Top-level normalisation       |
└───────────────────────────────*/

func normalize(raw any) (any, error) {
	switch v := raw.(type) {
	case *redis.SliceCmd:
		return v.Val(), nil
	case []interface{}:
		return v, nil
	case map[string]interface{}:
		return v, nil
	case map[interface{}]interface{}:
		// convert to string-keyed map
		m := make(map[string]interface{}, len(v))
		for k, val := range v {
			m[toStr(k)] = val
		}
		return m, nil
	default:
		return nil, fmt.Errorf("scan: unsupported reply type %T", raw)
	}
}

/*───────────────────────────────
|  Extract document hits         |
└───────────────────────────────*/

// Returns: totalResults, sliceOfHits, error.
func extractHits(reply any) (int, []any, error) {
	// RESP-3: top-level map
	if top, ok := reply.(map[string]interface{}); ok {
		resultsRaw, ok := top["results"].([]interface{})
		if !ok {
			return 0, nil, errors.New("scan: missing results array")
		}
		hits := make([]any, len(resultsRaw))
		for i, r := range resultsRaw {
			// Convert hit to string-keyed map
			var hit map[string]interface{}
			switch h := r.(type) {
			case map[string]interface{}:
				hit = h
			case map[interface{}]interface{}:
				hit = make(map[string]interface{}, len(h))
				for k, v := range h {
					hit[toStr(k)] = v
				}
			default:
				return 0, nil, fmt.Errorf("scan: unknown hit type %T", r)
			}
			if ea, ok := hit["extra_attributes"]; ok {
				hits[i] = ea
			} else if vals, ok := hit["values"]; ok { // old RETURN * style
				hits[i] = vals
			} else {
				hits[i] = hit
			}
		}

		total := len(hits)
		/*
			if tv, ok := top["total_results"]; ok {
				if n, ok := toInt64(tv); ok {
					total = int(n)
				}
			}
		*/
		return total, hits, nil
	}

	// RESP-2 / array form
	arr, ok := reply.([]interface{})
	if !ok {
		return 0, nil, fmt.Errorf("scan: unrecognised reply %T", reply)
	}
	if len(arr) == 0 {
		return 0, nil, nil
	}
	count, ok := arr[0].(int64)
	if !ok {
		return 0, nil, errors.New("scan: first array element is not int64")
	}
	total := int(count)
	hits := make([]any, total)
	for i := 0; i < total; i++ {
		hits[i] = arr[i*2+2] // skip doc-id elements
	}
	return total, hits, nil
}

/*───────────────────────────────
|  KV payload → map              |
└───────────────────────────────*/

func toStrMap(v any) (map[string]string, error) {
	switch t := v.(type) {
	case []interface{}: // RESP-2 KV list
		m := make(map[string]string, len(t)/2)
		for i := 0; i+1 < len(t); i += 2 {
			m[toStr(t[i])] = toStr(t[i+1])
		}
		return m, nil

	case map[interface{}]interface{}: // RESP-3 extra_attributes
		m := make(map[string]string, len(t))
		for k, v := range t {
			m[toStr(k)] = toStr(v)
		}
		return m, nil

	case map[string]interface{}:
		m := make(map[string]string, len(t))
		for k, v := range t {
			m[k] = toStr(v)
		}
		return m, nil

	default:
		return nil, fmt.Errorf("scan: unsupported kv type %T", v)
	}
}

/*───────────────────────────────
|  Struct assignment w/ cache    |
└───────────────────────────────*/

var metaCache sync.Map // reflect.Type → []fieldMeta

type fieldMeta struct {
	name  string
	index []int
	kind  reflect.Kind
}

func assign[T any](ptr *T, kv map[string]string) error {
	// fast-path: target is map[string]string
	var zero T
	if _, ok := any(zero).(map[string]string); ok {
		*ptr = any(kv).(T)
		return nil
	}

	val := reflect.ValueOf(ptr).Elem()
	rt := val.Type()

	metaAny, _ := metaCache.Load(rt)
	if metaAny == nil {
		metaAny = buildMeta(rt)
		metaCache.Store(rt, metaAny)
	}
	for _, fm := range metaAny.([]fieldMeta) {
		if s, ok := kv[fm.name]; ok {
			f := val.FieldByIndex(fm.index)
			switch fm.kind {
			case reflect.String:
				f.SetString(s)
			case reflect.Int, reflect.Int64, reflect.Int32:
				if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
					f.SetInt(n)
				}
			case reflect.Float32, reflect.Float64:
				if fl, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
					f.SetFloat(fl)
				}
			case reflect.Bool:
				f.SetBool(s == "1" || strings.EqualFold(s, "true"))
			}
		}
	}
	return nil
}

func buildMeta(rt reflect.Type) []fieldMeta {
	out := make([]fieldMeta, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		tag := f.Tag.Get("redisorm")
		if tag == "" {
			continue
		}
		name := strings.TrimPrefix(strings.Split(tag, ",")[0], "@")
		out = append(out, fieldMeta{name, f.Index, f.Type.Kind()})
	}
	return out
}

/*───────────────────────────────
|  Small util fns                |
└───────────────────────────────*/

func toStr(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []byte:
		return strings.TrimSpace(string(t))
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func toInt64(v interface{}) (int64, bool) {
	switch t := v.(type) {
	case int64:
		return t, true
	case int:
		return int64(t), true
	case float64:
		return int64(t), true
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return n, err == nil
	default:
		return 0, false
	}
}
