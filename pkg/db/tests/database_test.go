package db_test

import (
	"fmt"
	"testing"

	"dcs-lib/pkg/config"
	"dcs-lib/pkg/db"
	q "dcs-lib/pkg/db/queries"
	"dcs-lib/pkg/workorder"
)

func TestNewDb(t *testing.T) {
	err := config.Load("/home/terzivan/projects/MDOD/DCS/dcs-gradec/config/cfg.yaml")
	if err != nil {
		t.Fatal(fmt.Errorf("failed to load config:%w", err))
	}
	c := config.Provide()
	_, err = db.NewDatabase(c, "plant")
	if err != nil {
		t.Fatal(err)
	}
}

func TestOpenCloseDB(t *testing.T) {
	err := config.Load("/home/terzivan/projects/MDOD/DCS/dcs-gradec/config/cfg.yaml")
	if err != nil {
		t.Fatal(fmt.Errorf("failed to load config:%w", err))
	}
	c := config.Provide()
	pdb, err := db.NewDatabase(c, "plant")
	if err != nil {
		t.Fatal(err)
	}
	if err = pdb.Open(); err != nil {
		t.Fatal(err)
	}
	if err = pdb.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDbExecute(t *testing.T) {
	err := config.Load("/home/terzivan/projects/MDOD/DCS/dcs-gradec/config/cfg.yaml")
	if err != nil {
		t.Fatal(fmt.Errorf("failed to load config:%w", err))
	}
	c := config.Provide()
	database, err := db.NewDatabase(c, "plant")
	if err != nil {
		t.Fatal(err)
	}
	q, err := database.Query(q.NewQuery[q.GetWorkorderMeta](), 900923406)
	if err != nil {
		t.Fatal(err)
	}
	if do, ok := q.Unwrap().(workorder.DigestedOrder); !ok {
		t.Fatal("failed to unwrap query")
	} else {
		fmt.Printf("do: %+v\n", do)
	}
}
