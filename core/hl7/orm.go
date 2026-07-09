package hl7

import (
	"fmt"
	"strings"
	"time"
)

// ORMOrder represents an HL7 v2 ORM (Order) message for lab/imaging orders.
type ORMOrder struct {
	OrderControl    string // NW = new order, CA = cancel
	PlacerOrderID   string
	FillerOrderID   string
	PatientID       string // NHI
	PatientName     string
	DOB             string
	Gender          string
	OrderCode       string // LOINC or local code
	OrderText       string
	OrderStatus     string // pending, in-progress, completed, cancelled
	Priority        string // R = routine, S = stat, A = asap
	RequestedBy     string // HPI CPN
	OrderDateTime   time.Time
	Quantity        int
	Units           string
}

// BuildORM generates a raw HL7 v2 ORM^O01 message string.
func BuildORM(order ORMOrder) string {
	var segs []string

	priority := order.Priority
	if priority == "" {
		priority = "R"
	}
	quantity := order.Quantity
	if quantity == 0 {
		quantity = 1
	}

	// MSH
	segs = append(segs, fmt.Sprintf("MSH|^~\\&|TPT|NZ|LIS|NZ|%s||ORM^O01|MSG00001|P|2.5.1|||AL|NE",
		time.Now().Format("20060102150405")))

	// ORC — Common Order
	segs = append(segs, fmt.Sprintf("ORC|%s|%s|%s|||||%s|||%s|||||%s",
		order.OrderControl, order.PlacerOrderID, order.FillerOrderID,
		priority, order.RequestedBy, order.OrderDateTime.Format("20060102150405")))

	// PID — Patient
	segs = append(segs, fmt.Sprintf("PID|1||%s||%s||%s|%s",
		order.PatientID, order.PatientName, order.DOB, order.Gender))

	// OBR — Observation Request
	segs = append(segs, fmt.Sprintf("OBR|1|%s|%s||%s|%s|||||||||%s|||%s|||%s",
		order.PlacerOrderID, order.FillerOrderID,
		order.OrderCode, order.OrderText,
		order.RequestedBy, order.OrderDateTime.Format("20060102150405"),
		order.OrderStatus))

	// OBX — Quantity
	if order.Units != "" {
		segs = append(segs, fmt.Sprintf("OBX|1|NM|%s||%d|%s|||||||%s",
			order.OrderCode, quantity, order.Units, order.OrderStatus))
	}

	return strings.Join(segs, "\r")
}

// BuildORMNewOrder creates a new lab/imaging order.
func BuildORMNewOrder(patientID, name, dob, gender, orderCode, orderText, placerID, requestedBy string) string {
	return BuildORM(ORMOrder{
		OrderControl:  "NW",
		PlacerOrderID: placerID,
		PatientID:     patientID,
		PatientName:   name,
		DOB:           dob,
		Gender:        gender,
		OrderCode:     orderCode,
		OrderText:     orderText,
		OrderStatus:   "pending",
		Priority:      "R",
		RequestedBy:   requestedBy,
		OrderDateTime: time.Now(),
	})
}

// BuildORMCancel creates a cancel order.
func BuildORMCancel(placerID, fillerID, requestedBy string) string {
	return BuildORM(ORMOrder{
		OrderControl:  "CA",
		PlacerOrderID: placerID,
		FillerOrderID: fillerID,
		RequestedBy:   requestedBy,
		OrderDateTime: time.Now(),
	})
}
