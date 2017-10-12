// Package metadatautil provides getter and setter methods for metadata
package metadatautil

import (
	"time"

	"github.com/boltdb/bolt"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/pkg/errors"
)

// Fields to be added:
// Size int64
// AccessTime int64
// Tags
// Descr
// CachePolicy

const SizeUnknown int64 = -1
const keySize = "snapshot.size"
const keyEqualMutable = "cache.equalMutable"
const keyCachePolicy = "cache.cachePolicy"
const keyDescription = "cache.description"
const keyCreatedAt = "cache.createdAt"
const keyLastUsedAt = "cache.lastUsedAt"
const keyUsageCount = "cache.usageCount"

type CachePolicy int

const (
	CachePolicyDefault CachePolicy = iota
	CachePolicyRetain
)

func SetSize(si *metadata.StorageItem, s int64) error {
	v, err := metadata.NewValue(s)
	if err != nil {
		return errors.Wrap(err, "failed to create size value")
	}
	si.Queue(func(b *bolt.Bucket) error {
		return si.SetValue(b, keySize, v)
	})
	return nil
}

func GetSize(si *metadata.StorageItem) int64 {
	v := si.Get(keySize)
	if v == nil {
		return SizeUnknown
	}
	var size int64
	if err := v.Unmarshal(&size); err != nil {
		return SizeUnknown
	}
	return size
}

func GetEqualMutable(si *metadata.StorageItem) string {
	v := si.Get(keyEqualMutable)
	if v == nil {
		return ""
	}
	var str string
	if err := v.Unmarshal(&str); err != nil {
		return ""
	}
	return str
}

func SetEqualMutable(si *metadata.StorageItem, s string) error {
	v, err := metadata.NewValue(s)
	if err != nil {
		return errors.Wrapf(err, "failed to create %s meta value", keyEqualMutable)
	}
	si.Queue(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyEqualMutable, v)
	})
	return nil
}

func ClearEqualMutable(si *metadata.StorageItem) error {
	si.Queue(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyEqualMutable, nil)
	})
	return nil
}

func QueueCachePolicy(si *metadata.StorageItem, p CachePolicy) error {
	v, err := metadata.NewValue(p)
	if err != nil {
		return errors.Wrap(err, "failed to create cachePolicy value")
	}
	si.Queue(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyCachePolicy, v)
	})
	return nil
}

func GetCachePolicy(si *metadata.StorageItem) CachePolicy {
	v := si.Get(keyCachePolicy)
	if v == nil {
		return CachePolicyDefault
	}
	var p CachePolicy
	if err := v.Unmarshal(&p); err != nil {
		return CachePolicyDefault
	}
	return p
}

func QueueDescription(si *metadata.StorageItem, descr string) error {
	v, err := metadata.NewValue(descr)
	if err != nil {
		return errors.Wrap(err, "failed to create description value")
	}
	si.Queue(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyDescription, v)
	})
	return nil
}

func GetDescription(si *metadata.StorageItem) string {
	v := si.Get(keyDescription)
	if v == nil {
		return ""
	}
	var str string
	if err := v.Unmarshal(&str); err != nil {
		return ""
	}
	return str
}

func QueueCreatedAt(si *metadata.StorageItem) error {
	v, err := metadata.NewValue(time.Now().UnixNano())
	if err != nil {
		return errors.Wrap(err, "failed to create createdAt value")
	}
	si.Queue(func(b *bolt.Bucket) error {
		return si.SetValue(b, keyCreatedAt, v)
	})
	return nil
}

func GetCreatedAt(si *metadata.StorageItem) time.Time {
	v := si.Get(keyCreatedAt)
	if v == nil {
		return time.Time{}
	}
	var tm int64
	if err := v.Unmarshal(&tm); err != nil {
		return time.Time{}
	}
	return time.Unix(tm/1e9, tm%1e9)
}

func GetLastUsed(si *metadata.StorageItem) (int, *time.Time) {
	v := si.Get(keyUsageCount)
	if v == nil {
		return 0, nil
	}
	var usageCount int
	if err := v.Unmarshal(&usageCount); err != nil {
		return 0, nil
	}
	v = si.Get(keyLastUsedAt)
	if v == nil {
		return usageCount, nil
	}
	var lastUsedTs int64
	if err := v.Unmarshal(&lastUsedTs); err != nil || lastUsedTs == 0 {
		return usageCount, nil
	}
	tm := time.Unix(lastUsedTs/1e9, lastUsedTs%1e9)
	return usageCount, &tm
}

func UpdateLastUsed(si *metadata.StorageItem) error {
	count, _ := GetLastUsed(si)
	count++

	v, err := metadata.NewValue(count)
	if err != nil {
		return errors.Wrap(err, "failed to create usageCount value")
	}
	v2, err := metadata.NewValue(time.Now().UnixNano())
	if err != nil {
		return errors.Wrap(err, "failed to create lastUsedAt value")
	}
	return si.Update(func(b *bolt.Bucket) error {
		if err := si.SetValue(b, keyUsageCount, v); err != nil {
			return err
		}
		return si.SetValue(b, keyLastUsedAt, v2)
	})
}
