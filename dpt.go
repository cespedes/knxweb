package main

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/vapourismo/knx-go/knx/dpt"
)

// GetDPT returns the text representation of the internal value stored in a dpt.DatapointValue
func GetDPTAsString(v dpt.DatapointValue) string {
	Val := reflect.ValueOf(v)
	if Val.Kind() != reflect.Ptr {
		return "" // Error: input value is not a pointer
	}
	Val = Val.Elem()
	switch Val.Kind() {
	case reflect.Bool:
		return strconv.FormatBool(Val.Elem().Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprint(Val.Elem().Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprint(Val.Elem().Uint())
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%f", Val.Elem().Float())
	default:
		return fmt.Sprint(v.Pack())
	}
}

// GetDPT returns the value stored in a dpt.DatapointValue as a float64.
// It works only if its underlying type is a bool, integer or float.
func GetDPT(v dpt.DatapointValue) (float64, error) {
	Val := reflect.ValueOf(v)
	if Val.Kind() != reflect.Ptr {
		return 0.0, fmt.Errorf("GetDPT: input value is not a pointer")
	}
	Val = Val.Elem()
	switch Val.Kind() {
	case reflect.Bool:
		if Val.Elem().Bool() {
			return 1.0, nil
		}
		return 0.0, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(Val.Elem().Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(Val.Elem().Uint()), nil
	case reflect.Float32, reflect.Float64:
		return Val.Elem().Float(), nil
	default:
		return 0.0, fmt.Errorf("GetDPT: cannot get element: underlying type is %v", Val.Kind())
	}
}

// SetDPT sets the internal value of d to value.  Its kind must be integer, float or bool.
func SetDPT(d dpt.DatapointValue, value float64) error {
	Val := reflect.ValueOf(d)
	if Val.Kind() != reflect.Ptr {
		return fmt.Errorf("SetDPT: input variable is not a pointer")
	}
	if !Val.Elem().CanSet() {
		return fmt.Errorf("SetDPT: cannot set element value")
	}
	switch Val.Elem().Kind() {
	case reflect.Bool:
		Val.Elem().SetBool(value != 0.0)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		Val.Elem().SetInt(int64(value))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		Val.Elem().SetUint(uint64(value))
	case reflect.Float32, reflect.Float64:
		Val.Elem().SetFloat(value)
	default:
		return fmt.Errorf("SetDPT: cannot set element (underlying type %v)", Val.Elem().Kind())
	}

	// Normalize:
	d.Unpack(d.Pack())

	return nil
}

// SetDPT sets the internal value of d to value.  Its kind must be integer, float or bool.
func SetDPTFromString(d dpt.DatapointValue, value string) error {
	Val := reflect.ValueOf(d)
	if Val.Kind() != reflect.Ptr {
		return fmt.Errorf("SetDPT: input variable is not a pointer")
	}
	if !Val.Elem().CanSet() {
		return fmt.Errorf("SetDPT: cannot set element value")
	}
	switch Val.Elem().Kind() {
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		Val.Elem().SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		Val.Elem().SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		Val.Elem().SetUint(u)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		Val.Elem().SetFloat(f)
	default:
		return fmt.Errorf("SetDPT: cannot set element (underlying type %v)", Val.Elem().Kind())
	}

	// Normalize:
	d.Unpack(d.Pack())
	return nil
}

// NewDPT creates a dpt.DatapointValue of a given type with its internal value set as Value.
// It works if its underlying type is bool, float or integer.
func NewDPT(dptType string, value float64) (dpt.DatapointValue, error) {
	v, ok := dpt.Produce(dptType)
	if !ok {
		return nil, fmt.Errorf("NewDPT: invalid KNX type %q", dptType)
	}
	err := SetDPT(v, value)
	return v, err
}
