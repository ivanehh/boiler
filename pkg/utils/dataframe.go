package utils

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/ivanehh/boiler/pkg/logging"
	"github.com/pbnjay/grate"
	_ "github.com/pbnjay/grate/simple"
	_ "github.com/pbnjay/grate/xls"
)

var l *logging.DCSlogger

type (
	DfOpts       func(d *Dataframe) error
	BadDataframe struct{}
)

type Dataframe struct {
	Columns          []Column
	Rows             []Record
	interpretColumns bool
}

// DfRowsAsStructList the dataframe as a []sType representation; sType must have 'df' tags
func DfRowsAsStructList[sType any](d *Dataframe) ([]sType, error) {
	var err error
	result := make([]sType, len(d.Rows))
	rPointers := make([]*sType, len(d.Rows))
	for idx := range rPointers {
		rPointers[idx] = new(sType)
	}
	for idx, s := range rPointers {
		sValue := reflect.ValueOf(s).Elem()
		sType := sValue.Type()
		for i := 0; i < sValue.NumField(); i++ {
			field := sValue.Field(i)
			fieldTag := strings.ToLower(sType.Field(i).Tag.Get("df"))
			if len(fieldTag) == 0 || fieldTag == "-" {
				continue
			}
			if !slices.Contains(d.Header(), fieldTag) {
				l.Warn("header-mismatch", fmt.Errorf("%s not found in %v", fieldTag, d.Header()))
				continue
			}
			for cid := range d.Columns {
				if d.Columns[cid].name == fieldTag {
					switch field.Kind() {
					case reflect.String:
						field.SetString(d.Rows[idx][cid])
						rPointers[idx] = s
					case reflect.Float64:
						var fv float64
						fv, err = strconv.ParseFloat(d.Rows[idx][cid], 64)
						if err != nil {
							return nil, err
						}
						field.SetFloat(fv)
						rPointers[idx] = s
					}
					break
				}
			}
		}
	}
	for idx, r := range rPointers {
		result[idx] = *r
	}
	return result, nil
}

type Column struct {
	name    string
	idx     int
	content []string
}

type Record []string

func (d *Dataframe) Header() []string {
	header := make([]string, len(d.Columns))
	for i := range d.Columns {
		header[i] = d.Columns[i].name
	}
	return header
}

func WithRecordsFromText(b []byte, newLine string, sep string) DfOpts {
	return func(d *Dataframe) error {
		csvRecords := bytes.Split(b, []byte(newLine))
		for _, r := range csvRecords {
			dfRecord := make(Record, 0)
			csvValues := bytes.Split(r, []byte(sep))
			for _, v := range csvValues {
				dfRecord = append(dfRecord, string(v))
			}
			d.Rows = append(d.Rows, dfRecord)
		}
		return nil
	}
}

func WithRecordsFromFiles(filePaths []string) DfOpts {
	return func(d *Dataframe) error {
		var head []string
		for idx, fp := range filePaths {
			source, err := grate.Open(fp)
			if err != nil {
				return err
			}
			sheets, err := source.List()
			if err != nil {
				return err
			}
			data, err := source.Get(sheets[0])
			if err != nil {
				return err
			}
			/*
				this part is a bit awkward
				if we are not at the first file then we want to skip the header
			*/
			if idx != 0 {
				for data.Next() {
					// advance rows as long as they are empty
					if len(data.Strings()[0]) < 1 || len(data.Strings()[0]) == 0 {
						continue
					}
					// do not generate dataframe for file sets that do not have identical headers
					if head != nil {
						var cr Record
						r := data.Strings()
						if strings.Contains(r[0], ",") {
							if cr = cleanRecord(strings.Split(r[0], ",")); len(cr) > 0 {
								if slices.Compare(head, cr) != 0 {
									return &HeaderMismatchErr{
										Original: head,
										Mismatch: cr,
									}
								}
							}
						} else {
							if cr = cleanRecord(r); len(cr) > 0 {
								if slices.Compare(head, cr) != 0 {
									return &HeaderMismatchErr{
										Original: head,
										Mismatch: cr,
									}
								}
							}
						}
					}
					break
				}
			}
			for data.Next() {
				r := data.Strings()
				var cr Record
				if strings.Contains(r[0], ",") {
					if cr = cleanRecord(strings.Split(r[0], ",")); len(cr) > 0 {
						d.Rows = append(d.Rows, cr)
					}
				} else {
					if cr = cleanRecord(r); len(cr) > 0 {
						d.Rows = append(d.Rows, cr)
					}
				}
				// set the default header for this dataframe
				if slices.ContainsFunc(cr, func(e string) bool {
					return strings.EqualFold(e, "date")
				}) && head == nil {
					head = cleanRecord(cr)
				}
			}
		}
		return nil
	}
}

func cleanRecord(r []string) Record {
	newR := make(Record, 0)
	for idx := range r {
		if len(r[idx]) > 0 {
			newR = append(newR, strings.Trim(r[idx], " +-"))
		}
	}
	return newR
}

func WithProvidedColumns(h []string) DfOpts {
	return func(d *Dataframe) error {
		err := interpretColumns(d, h)
		if err != nil {
			return err
		}
		return nil
	}
}

func WithInterpretedColumns() DfOpts {
	return func(d *Dataframe) error {
		d.interpretColumns = true
		return nil
	}
}

func interpretColumns(d *Dataframe, h []string) error {
	if r := slices.Compare(h, d.Rows[0]); r != 0 {
		// TODO: header mismatch error
		return &HeaderInterpretErr{Provided: h, Found: d.Rows[0]}
	}
	for idx, str := range h {
		d.Columns = append(d.Columns, Column{
			name:    strings.ToLower(strings.ReplaceAll(str, " ", "")),
			idx:     idx,
			content: make([]string, 0),
		})
	}
	d.Rows = d.Rows[1:]
	return nil
}

// Drop a range of rows from the dataframe
func (d *Dataframe) Drop(i ...int) {
	slices.Sort(i)
	d.Rows = slices.Delete(d.Rows, i[0], i[len(i)-1])
	d.clean()
}

func (d *Dataframe) Get(row int, columns ...string) (*Dataframe, error) {
	var r []string
	var result Record
	r = d.Rows[row]
	dnew := new(Dataframe)
	if len(columns) == 0 {
		dnew.Columns = d.Columns
		dnew.Rows = []Record{d.Rows[row]}
		return dnew, nil
	}
	for _, c := range d.Columns {
		// if slices.Contains(columns, c.name) {
		// 	result = append(result, r[c.idx])
		// 	dnew.Columns = append(dnew.Columns, c)
		// }
		if slices.ContainsFunc(columns, func(e string) bool {
			return func(dfc string) bool {
				return strings.EqualFold(e, dfc)
			}(c.name)
		}) {
			result = append(result, r[c.idx])
			dnew.Columns = append(dnew.Columns, c)
		}
	}
	if len(dnew.Columns) != len(columns) {
		return nil, &ColumnsNotFoundErr{
			Available: d.Header(),
			Required:  columns,
		}
	}
	dnew.Rows = []Record{result}
	return dnew, nil
}

func (d *Dataframe) clean() {
	dfWidth := len(d.Columns)
	cleanRecords := make([]Record, 0)
	for _, r := range d.Rows {
		// we want all records to be with the same length as the dataframe header AND sometimes we have headers in the middle of our files :)
		if len(r) == dfWidth && !strings.EqualFold(d.Header()[0], cleanRecord(r)[0]) {
			cleanRecords = append(cleanRecords, r)
		}
	}
	d.Rows = cleanRecords
}

func NewDataframe(opts ...DfOpts) (*Dataframe, error) {
	l = logging.Provide()
	df := new(Dataframe)
	for _, opt := range opts {
		err := opt(df)
		// TODO: Should we quit dataframe construction and return if a dataframe opt fails?
		if err != nil {
			return nil, err
		}
	}
	if df.interpretColumns {
		interpretColumns(df, df.Rows[0])
	}
	df.clean()
	return df, nil
}
