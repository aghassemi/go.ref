// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ble

import (
	"github.com/paypal/gatt"
	"github.com/paypal/gatt/linux/cmd"
)

var (
	// These constants are taken from:
	// https://developer.bluetooth.org/gatt/characteristics/Pages/CharacteristicsHome.aspx
	attrDeviceNameUUID        = gatt.UUID16(0x2A00)
	attrAppearanceUUID        = gatt.UUID16(0x2A01)
	attrPeripheralPrivacyUUID = gatt.UUID16(0x2A02)
	attrReconnectionAddrUUID  = gatt.UUID16(0x2A03)
	attrPreferredParamsUUID   = gatt.UUID16(0x2A04)

	attrServiceChangedUUID = gatt.UUID16(0x2A05)

	// https://developer.bluetooth.org/gatt/characteristics/Pages/CharacteristicViewer.aspx?u=org.bluetooth.characteristic.gap.appearance.xml
	gapCharAppearanceGenericComputer = []byte{0x00, 0x80}
)

func newGapService(name string) *gatt.Service {
	s := gatt.NewService(attrGAPUUID)
	s.AddCharacteristic(attrDeviceNameUUID).SetValue([]byte(name))
	s.AddCharacteristic(attrAppearanceUUID).SetValue(gapCharAppearanceGenericComputer)
	// Disable peripheral privacy
	s.AddCharacteristic(attrPeripheralPrivacyUUID).SetValue([]byte{0x00})

	// Make up some value for a required field:
	// https://developer.bluetooth.org/gatt/characteristics/Pages/CharacteristicsHome.aspx
	s.AddCharacteristic(attrReconnectionAddrUUID).SetValue([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	// Preferred params are:
	//   min connection interval: 0x0006 * 1.25ms (7.5 ms)
	//   max connection interval: 0x0006 * 1.25ms (7.5 ms)
	//   slave latency: 0
	//   connection supervision timeout multiplier: 0x07d0 (2000)
	s.AddCharacteristic(attrPreferredParamsUUID).SetValue([]byte{0x06, 0x00, 0x06, 0x00, 0x00, 0x00, 0xd0, 0x07})
	return s
}

func newGattService() *gatt.Service {
	s := gatt.NewService(attrGATTUUID)
	s.AddCharacteristic(attrServiceChangedUUID).HandleNotifyFunc(
		func(r gatt.Request, n gatt.Notifier) {})
	return s
}

var gattOptions = []gatt.Option{
	gatt.LnxMaxConnections(1),
	gatt.LnxDeviceID(-1, true),
	gatt.LnxSetAdvertisingParameters(&cmd.LESetAdvertisingParameters{
		// Set an advertising rate of 150ms.  This value is multipled by
		// 0.625ms to get the actual rate.
		AdvertisingIntervalMin: 0x00f4,
		AdvertisingIntervalMax: 0x00f4,
		AdvertisingChannelMap:  0x7,
	}),
}

func addDefaultServices(name string) map[string]*gatt.Service {
	s1 := newGapService(name)
	s2 := newGattService()
	return map[string]*gatt.Service{
		s1.UUID().String(): s1,
		s2.UUID().String(): s2,
	}
}