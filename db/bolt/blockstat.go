package bolt

import (
	"bytes"
	"encoding/binary"
	"time"

	est "github.com/bitcoinfees/feesim/estimate"
	"github.com/boltdb/bolt"
)

type blockstatdb struct {
	db          *bolt.DB
	byteOrder   binary.ByteOrder
	statsBucket []byte
}

func LoadBlockStatDB(dbfile string) (*blockstatdb, error) {
	db, err := bolt.Open(dbfile, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}
	d := &blockstatdb{
		db:          db,
		byteOrder:   binary.BigEndian,
		statsBucket: []byte("blockstats"),
	}
	err = d.db.Update(func(tr *bolt.Tx) error {
		_, err = tr.CreateBucketIfNotExists(d.statsBucket)
		return err
	})
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *blockstatdb) Get(start, end int64) ([]*est.BlockStat, error) {
	var stats []*est.BlockStat
	err := d.db.View(func(tr *bolt.Tx) error {
		bkt := tr.Bucket(d.statsBucket)
		c := bkt.Cursor()
		startkey, endkey := itob(start), itob(end)
		for k, v := c.Seek(startkey); k != nil && bytes.Compare(k, endkey) <= 0; k, v = c.Next() {
			b := new(est.BlockStat)
			if err := binary.Read(bytes.NewBuffer(v), d.byteOrder, b); err != nil {
				return err
			}
			stats = append(stats, b)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return stats, nil
}

func (d *blockstatdb) Put(b []*est.BlockStat) error {
	err := d.db.Update(func(tr *bolt.Tx) error {
		bkt := tr.Bucket(d.statsBucket)
		for _, bi := range b {
			key := itob(bi.Height)
			value := new(bytes.Buffer)
			if err := binary.Write(value, d.byteOrder, bi); err != nil {
				return err
			}
			if err := bkt.Put(key, value.Bytes()); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (d *blockstatdb) Delete(start, end int64) error {
	err := d.db.Update(func(tr *bolt.Tx) error {
		b := tr.Bucket(d.statsBucket)
		c := b.Cursor()
		startkey, endkey := itob(start), itob(end)
		var del [][]byte
		for k, _ := c.Seek(startkey); k != nil && bytes.Compare(k, endkey) <= 0; k, _ = c.Next() {
			del = append(del, k)
		}
		for _, k := range del {
			if err := b.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (d *blockstatdb) Close() error {
	return d.db.Close()
}
