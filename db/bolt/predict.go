package bolt

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"time"

	"github.com/boltdb/bolt"

	"github.com/bitcoinfees/feesim/predict"
)

type predictdb struct {
	db           *bolt.DB
	byteOrder    binary.ByteOrder
	txBucket     []byte
	countsBucket []byte
}

func LoadPredictDB(dbfile string) (*predictdb, error) {
	db, err := bolt.Open(dbfile, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}
	d := &predictdb{
		db:           db,
		byteOrder:    binary.BigEndian,
		txBucket:     []byte("tx"),
		countsBucket: []byte("counts"),
	}
	err = d.db.Update(func(tr *bolt.Tx) error {
		if _, err := tr.CreateBucketIfNotExists(d.txBucket); err != nil {
			return err
		}
		if _, err := tr.CreateBucketIfNotExists(d.countsBucket); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (d *predictdb) GetTxs(txids []string) (map[string]predict.Tx, error) {
	txs := make(map[string]predict.Tx)
	err := d.db.View(func(tr *bolt.Tx) error {
		bkt := tr.Bucket(d.txBucket)
		for _, txid := range txids {
			v := bkt.Get([]byte(txid))
			if v == nil {
				continue
			}
			var tx predict.Tx
			if err := binary.Read(bytes.NewBuffer(v), d.byteOrder, &tx); err != nil {
				return err
			}
			txs[txid] = tx
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return txs, nil
}

func (d *predictdb) PutTxs(txs map[string]predict.Tx) error {
	err := d.db.Update(func(tr *bolt.Tx) error {
		bkt := tr.Bucket(d.txBucket)
		for txid, tx := range txs {
			buf := new(bytes.Buffer)
			if err := binary.Write(buf, d.byteOrder, tx); err != nil {
				return err
			}
			if err := bkt.Put([]byte(txid), buf.Bytes()); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (d *predictdb) GetScores() (attained, exceeded []float64, err error) {
	err = d.db.View(func(tr *bolt.Tx) error {
		bkt := tr.Bucket(d.countsBucket)
		if v := bkt.Get([]byte("attained")); v != nil {
			buf := bytes.NewBuffer(v)
			if err := gob.NewDecoder(buf).Decode(&attained); err != nil {
				return err
			}
		}
		if v := bkt.Get([]byte("exceeded")); v != nil {
			buf := bytes.NewBuffer(v)
			if err := gob.NewDecoder(buf).Decode(&exceeded); err != nil {
				return err
			}
		}
		return nil
	})
	return
}

func (d *predictdb) PutScores(attained, exceeded []float64) error {
	err := d.db.Update(func(tr *bolt.Tx) error {
		bkt := tr.Bucket(d.countsBucket)
		buf := new(bytes.Buffer)
		if err := gob.NewEncoder(buf).Encode(attained); err != nil {
			return err
		}
		if err := bkt.Put([]byte("attained"), buf.Bytes()); err != nil {
			return err
		}

		buf = new(bytes.Buffer)
		if err := gob.NewEncoder(buf).Encode(exceeded); err != nil {
			return err
		}
		if err := bkt.Put([]byte("exceeded"), buf.Bytes()); err != nil {
			return err
		}

		return nil
	})
	return err
}

func (d *predictdb) Reconcile(txids []string) error {
	txidSet := make(map[string]bool)
	for _, txid := range txids {
		txidSet[txid] = true
	}
	err := d.db.Update(func(tr *bolt.Tx) error {
		var del [][]byte
		bkt := tr.Bucket(d.txBucket)
		err := bkt.ForEach(func(k, v []byte) error {
			if !txidSet[string(k)] {
				del = append(del, k)
			}
			return nil
		})
		if err != nil {
			return err
		}
		for _, k := range del {
			if err := bkt.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (d *predictdb) Close() error {
	return d.db.Close()
}
