// Package bolt contains implementations of the DB interfaces used by package
// main.
package bolt

import (
	"bytes"
	"encoding/binary"
	"sort"
	"time"

	est "github.com/bitcoinfees/feesim/estimate"
	"github.com/boltdb/bolt"
)

type txdb struct {
	db        *bolt.DB
	byteOrder binary.ByteOrder
	txBucket  []byte
}

func LoadTxDB(dbfile string) (*txdb, error) {
	db, err := bolt.Open(dbfile, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}
	d := &txdb{
		db:        db,
		byteOrder: binary.BigEndian,
		txBucket:  []byte("txs"),
	}

	err = d.db.Update(func(tr *bolt.Tx) error {
		_, err = tr.CreateBucketIfNotExists(d.txBucket)
		return err
	})
	if err != nil {
		return nil, err
	}
	return d, nil
}

// Get wraps get inside a View tx.
func (d *txdb) Get(start, end int64) ([]est.Tx, error) {
	var txs []est.Tx
	err := d.db.View(func(tr *bolt.Tx) error {
		var err error
		txs, err = d.get(start, end, tr)
		return err
	})
	return txs, err
}

// Put wraps put inside an Update tx.
func (d *txdb) Put(txs []est.Tx) error {
	estTxSlice(txs).Sort() // Faster Putting, I think
	return d.db.Update(func(tr *bolt.Tx) error {
		return d.put(txs, tr)
	})
}

// Delete deletes all txs with time in between start and end.
func (d *txdb) Delete(start, end int64) error {
	err := d.db.Update(func(tr *bolt.Tx) error {
		return d.delete(start, end, tr)
	})
	return err
}

func (d *txdb) Close() error {
	return d.db.Close()
}

// get returns a slice of all txs that have Time in between start and end.
func (d *txdb) get(start, end int64, tr *bolt.Tx) ([]est.Tx, error) {
	var txs []est.Tx
	b := tr.Bucket(d.txBucket)
	c := b.Cursor()
	startkey, endkey := itob(start), itob(end)
	for k, _ := c.Seek(startkey); k != nil && bytes.Compare(k, endkey) <= 0; k, _ = c.Next() {
		btx := b.Bucket(k)
		err := btx.ForEach(func(_ []byte, v []byte) error {
			// Append bucket contents to txs
			tx := est.Tx{}
			if err := binary.Read(bytes.NewBuffer(v), d.byteOrder, &tx); err != nil {
				return err
			}
			txs = append(txs, tx)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return txs, nil
}

func (d *txdb) put(txs []est.Tx, tr *bolt.Tx) error {
	b := tr.Bucket(d.txBucket)
	for _, tx := range txs {
		btx, err := b.CreateBucketIfNotExists(itob(tx.Time))
		if err != nil {
			return err
		}
		id, err := btx.NextSequence()
		if err != nil {
			return err
		}
		key := itob(int64(id)) // uint64 -> int64

		value := new(bytes.Buffer)
		if err := binary.Write(value, d.byteOrder, tx); err != nil {
			return err
		}
		if err := btx.Put(key, value.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

func (d *txdb) delete(start, end int64, tr *bolt.Tx) error {
	b := tr.Bucket(d.txBucket)
	c := b.Cursor()
	startkey, endkey := itob(start), itob(end)
	var del [][]byte
	for k, _ := c.Seek(startkey); k != nil && bytes.Compare(k, endkey) <= 0; k, _ = c.Next() {
		del = append(del, k)
	}
	for _, k := range del {
		if err := b.DeleteBucket(k); err != nil {
			return err
		}
	}
	return nil
}

// itob returns an 8-byte big endian representation of v.
// The input argument v must be positive.
func itob(v int64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

// btoi is the inverse of itob.
func btoi(b []byte) int64 {
	v := binary.BigEndian.Uint64(b)
	return int64(v)
}

// estTxSlice implements sort.Interface for []est.Tx
type estTxSlice []est.Tx

func (s estTxSlice) Len() int {
	return len(s)
}

func (s estTxSlice) Less(i, j int) bool {
	if s[i].Time != s[j].Time {
		return s[i].Time < s[j].Time
	}
	if s[i].Size != s[j].Size {
		return s[i].Size < s[j].Size
	}
	return s[i].FeeRate < s[j].FeeRate
}

func (s estTxSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s estTxSlice) Sort() {
	sort.Sort(s)
}
