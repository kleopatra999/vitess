// Code generated by protoc-gen-go.
// source: throttlerdata.proto
// DO NOT EDIT!

/*
Package throttlerdata is a generated protocol buffer package.

It is generated from these files:
	throttlerdata.proto

It has these top-level messages:
	MaxRatesRequest
	MaxRatesResponse
	SetMaxRateRequest
	SetMaxRateResponse
*/
package throttlerdata

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// MaxRatesRequest is the payload for the MaxRates RPC.
type MaxRatesRequest struct {
}

func (m *MaxRatesRequest) Reset()                    { *m = MaxRatesRequest{} }
func (m *MaxRatesRequest) String() string            { return proto.CompactTextString(m) }
func (*MaxRatesRequest) ProtoMessage()               {}
func (*MaxRatesRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

// MaxRatesResponse is returned by the MaxRates RPC.
type MaxRatesResponse struct {
	// max_rates returns the max rate for each throttler. It's keyed by the
	// throttler name.
	Rates map[string]int64 `protobuf:"bytes,1,rep,name=rates" json:"rates,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"varint,2,opt,name=value"`
}

func (m *MaxRatesResponse) Reset()                    { *m = MaxRatesResponse{} }
func (m *MaxRatesResponse) String() string            { return proto.CompactTextString(m) }
func (*MaxRatesResponse) ProtoMessage()               {}
func (*MaxRatesResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *MaxRatesResponse) GetRates() map[string]int64 {
	if m != nil {
		return m.Rates
	}
	return nil
}

// SetMaxRateRequest is the payload for the SetMaxRate RPC.
type SetMaxRateRequest struct {
	Rate int64 `protobuf:"varint,1,opt,name=rate" json:"rate,omitempty"`
}

func (m *SetMaxRateRequest) Reset()                    { *m = SetMaxRateRequest{} }
func (m *SetMaxRateRequest) String() string            { return proto.CompactTextString(m) }
func (*SetMaxRateRequest) ProtoMessage()               {}
func (*SetMaxRateRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

// SetMaxRateResponse is returned by the SetMaxRate RPC.
type SetMaxRateResponse struct {
	// names is the list of throttler names which were updated.
	Names []string `protobuf:"bytes,1,rep,name=names" json:"names,omitempty"`
}

func (m *SetMaxRateResponse) Reset()                    { *m = SetMaxRateResponse{} }
func (m *SetMaxRateResponse) String() string            { return proto.CompactTextString(m) }
func (*SetMaxRateResponse) ProtoMessage()               {}
func (*SetMaxRateResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

func init() {
	proto.RegisterType((*MaxRatesRequest)(nil), "throttlerdata.MaxRatesRequest")
	proto.RegisterType((*MaxRatesResponse)(nil), "throttlerdata.MaxRatesResponse")
	proto.RegisterType((*SetMaxRateRequest)(nil), "throttlerdata.SetMaxRateRequest")
	proto.RegisterType((*SetMaxRateResponse)(nil), "throttlerdata.SetMaxRateResponse")
}

func init() { proto.RegisterFile("throttlerdata.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 204 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0xe2, 0x12, 0x2e, 0xc9, 0x28, 0xca,
	0x2f, 0x29, 0xc9, 0x49, 0x2d, 0x4a, 0x49, 0x2c, 0x49, 0xd4, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17,
	0xe2, 0x45, 0x11, 0x54, 0x12, 0xe4, 0xe2, 0xf7, 0x4d, 0xac, 0x08, 0x4a, 0x2c, 0x49, 0x2d, 0x0e,
	0x4a, 0x2d, 0x2c, 0x4d, 0x2d, 0x2e, 0x51, 0xea, 0x63, 0xe4, 0x12, 0x40, 0x88, 0x15, 0x17, 0xe4,
	0xe7, 0x15, 0xa7, 0x0a, 0x39, 0x70, 0xb1, 0x16, 0x81, 0x04, 0x24, 0x18, 0x15, 0x98, 0x35, 0xb8,
	0x8d, 0xb4, 0xf4, 0x50, 0xcd, 0x46, 0x57, 0xaf, 0x07, 0xe6, 0xb9, 0xe6, 0x95, 0x14, 0x55, 0x06,
	0x41, 0x34, 0x4a, 0x59, 0x70, 0x71, 0x21, 0x04, 0x85, 0x04, 0xb8, 0x98, 0xb3, 0x53, 0x2b, 0x25,
	0x18, 0x15, 0x18, 0x35, 0x38, 0x83, 0x40, 0x4c, 0x21, 0x11, 0x2e, 0xd6, 0xb2, 0xc4, 0x9c, 0xd2,
	0x54, 0x09, 0x26, 0x05, 0x46, 0x0d, 0xe6, 0x20, 0x08, 0xc7, 0x8a, 0xc9, 0x82, 0x51, 0x49, 0x9d,
	0x4b, 0x30, 0x38, 0xb5, 0x04, 0x6a, 0x05, 0xd4, 0x95, 0x42, 0x42, 0x5c, 0x2c, 0x20, 0x73, 0xc1,
	0x26, 0x30, 0x07, 0x81, 0xd9, 0x4a, 0x5a, 0x5c, 0x42, 0xc8, 0x0a, 0xa1, 0x4e, 0x17, 0xe1, 0x62,
	0xcd, 0x4b, 0xcc, 0x85, 0x3a, 0x9d, 0x33, 0x08, 0xc2, 0x49, 0x62, 0x03, 0x07, 0x87, 0x31, 0x20,
	0x00, 0x00, 0xff, 0xff, 0x27, 0x1c, 0xbc, 0x61, 0x25, 0x01, 0x00, 0x00,
}
