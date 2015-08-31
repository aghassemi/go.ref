// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package demodb

import (
	"fmt"
	"time"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase/nosql"
	"v.io/v23/syncbase/nosql"
	"v.io/v23/vdl"
)

type kv struct {
	key   string
	value *vdl.Value
}

type table struct {
	name string
	rows []kv
}

const demoPrefix = "Demo"

var demoTables = []table{
	table{
		name: "Customers",
		rows: []kv{
			kv{
				"001",
				vdl.ValueOf(Customer{"John Smith", 1, true, AddressInfo{"1 Main St.", "Palo Alto", "CA", "94303"}, CreditReport{Agency: CreditAgencyEquifax, Report: AgencyReportEquifaxReport{EquifaxCreditReport{'A'}}}}),
			},
			kv{
				"001001",
				vdl.ValueOf(Invoice{1, 1000, 42, AddressInfo{"1 Main St.", "Palo Alto", "CA", "94303"}}),
			},
			kv{
				"001002",
				vdl.ValueOf(Invoice{1, 1003, 7, AddressInfo{"2 Main St.", "Palo Alto", "CA", "94303"}}),
			},
			kv{
				"001003",
				vdl.ValueOf(Invoice{1, 1005, 88, AddressInfo{"3 Main St.", "Palo Alto", "CA", "94303"}}),
			},
			kv{
				"002",
				vdl.ValueOf(Customer{"Bat Masterson", 2, true, AddressInfo{"777 Any St.", "Collins", "IA", "50055"}, CreditReport{Agency: CreditAgencyTransUnion, Report: AgencyReportTransUnionReport{TransUnionCreditReport{80}}}}),
			},
			kv{
				"002001",
				vdl.ValueOf(Invoice{2, 1001, 166, AddressInfo{"777 Any St.", "Collins", "IA", "50055"}}),
			},
			kv{
				"002002",
				vdl.ValueOf(Invoice{2, 1002, 243, AddressInfo{"888 Any St.", "Collins", "IA", "50055"}}),
			},
			kv{
				"002003",
				vdl.ValueOf(Invoice{2, 1004, 787, AddressInfo{"999 Any St.", "Collins", "IA", "50055"}}),
			},
			kv{
				"002004",
				vdl.ValueOf(Invoice{2, 1006, 88, AddressInfo{"101010 Any St.", "Collins", "IA", "50055"}}),
			},
		},
	},
	table{
		name: "Numbers",
		rows: []kv{
			kv{
				"001",
				vdl.ValueOf(Numbers{byte(12), uint16(1234), uint32(5678), uint64(999888777666), int16(9876), int32(876543), int64(128), float32(3.14159), float64(2.71828182846), complex64(123.0 + 7.0i), complex128(456.789 + 10.1112i)}),
			},
			kv{
				"002",
				vdl.ValueOf(Numbers{byte(9), uint16(99), uint32(999), uint64(9999999), int16(9), int32(99), int64(88), float32(1.41421356237), float64(1.73205080757), complex64(9.87 + 7.65i), complex128(4.32 + 1.0i)}),
			},
			kv{
				"003",
				vdl.ValueOf(Numbers{byte(210), uint16(210), uint32(210), uint64(210), int16(210), int32(210), int64(210), float32(210.0), float64(210.0), complex64(210.0 + 0.0i), complex128(210.0 + 0.0i)}),
			},
		},
	},
	table{
		name: "Composites",
		rows: []kv{
			kv{
				"uno",
				vdl.ValueOf(Composite{Array2String{"foo", "bar"}, []int32{1, 2}, map[int32]struct{}{1: struct{}{}, 2: struct{}{}}, map[string]int32{"foo": 1, "bar": 2}}),
			},
		},
	},
	table{
		name: "Recursives",
		rows: []kv{
			kv{
				"alpha",
				vdl.ValueOf(Recursive{nil, &Times{time.Unix(123456789, 42244224), time.Duration(1337)}, map[Array2String]Recursive{
					Array2String{"a", "b"}: Recursive{},
					Array2String{"x", "y"}: Recursive{vdl.ValueOf(CreditReport{Agency: CreditAgencyExperian, Report: AgencyReportExperianReport{ExperianCreditReport{ExperianRatingGood}}}), nil, map[Array2String]Recursive{
						Array2String{"alpha", "beta"}: Recursive{vdl.ValueOf(FooType{Bar: BarType{Baz: BazType{Name: "hello", TitleOrValue: TitleOrValueTypeValue{Value: 42}}}}), nil, nil},
					}},
					Array2String{"u", "v"}: Recursive{vdl.ValueOf(vdl.TypeOf(Recursive{})), nil, nil},
				}}),
			},
		},
	},
}

// Creates demo tables in the provided database. Tables are deleted and
// recreated if they already exist.
func PopulateDemoDB(ctx *context.T, db nosql.Database) error {
	for i, t := range demoTables {
		tn := demoPrefix + t.name
		if err := db.DeleteTable(ctx, tn); err != nil {
			return fmt.Errorf("failed deleting table %s (%d/%d): %v", tn, i+1, len(demoTables), err)
		}
		if err := db.CreateTable(ctx, tn, nil); err != nil {
			return fmt.Errorf("failed creating table %s (%d/%d): %v", tn, i+1, len(demoTables), err)
		}
		if err := nosql.RunInBatch(ctx, db, wire.BatchOptions{}, func(db nosql.BatchDatabase) error {
			dt := db.Table(tn)
			for _, kv := range t.rows {
				if err := dt.Put(ctx, kv.key, kv.value); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("failed populating table %s (%d/%d): %v", tn, i+1, len(demoTables), err)
		}
	}
	return nil
}
