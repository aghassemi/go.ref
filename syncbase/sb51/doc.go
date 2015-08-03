// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Antimony (sb51) is a Syncbase general-purpose client and management utility.
// It currently supports experimenting with the Syncbase query language.
//
// The 'sh' command connects to a specified database on a Syncbase instance,
// creating it if it does not exist if -create-missing is specified.
// The user can then enter the following at the command line:
//     1. dump - to get a dump of the database
//     2. a syncbase select statement - which is executed and results printed to stdout
//     3. make-demo - to create demo tables in the database to experiment with, equivalent to -make-demo flag
//     4. exit (or quit) - to exit the program
//
// When the shell is running non-interactively (stdin not connected to a tty),
// errors cause the shell to exit with a non-zero status.
//
// To build client:
//     v23 go install v.io/syncbase/x/ref/syncbase/sb51
//
// To run client:
//     $V23_ROOT/roadmap/go/bin/sb51 sh <appname> <dbname>
//
// Sample run (assuming a syncbase service is mounted at '/:8101/syncbase',
// otherwise specify using -service flag):
//     > $V23_ROOT/roadmap/go/bin/sb51 sh -create-missing -make-demo -format=csv demoapp demodb
//     ? select v.Name, v.Address.State from DemoCustomers where Type(v) = "Customer";
//     v.Name,v.Address.State
//     John Smith,CA
//     Bat Masterson,IA
//     ? select v.CustId, v.InvoiceNum, v.ShipTo.Zip, v.Amount from DemoCustomers where Type(v) = "Invoice" and v.Amount > 100;
//     v.CustId,v.InvoiceNum,v.ShipTo.Zip,v.Amount
//     2,1001,50055,166
//     2,1002,50055,243
//     2,1004,50055,787
//     ? select k, v fro DemoCustomers;
//     Error:
//     select k, v fro DemoCustomers
//                 ^
//     13: Expected 'from', found fro.
//     ? select k, v from DemoCustomers;
//     k,v
//     001,"{Name: ""John Smith"", Id: 1, Active: true, Address: {Street: ""1 Main St."", City: ""Palo Alto"", State: ""CA"", Zip: ""94303""}, Credit: {Agency: Equifax, Report: EquifaxReport: {Rating: 65}}}"
//     001001,"{CustId: 1, InvoiceNum: 1000, Amount: 42, ShipTo: {Street: ""1 Main St."", City: ""Palo Alto"", State: ""CA"", Zip: ""94303""}}"
//     001002,"{CustId: 1, InvoiceNum: 1003, Amount: 7, ShipTo: {Street: ""2 Main St."", City: ""Palo Alto"", State: ""CA"", Zip: ""94303""}}"
//     001003,"{CustId: 1, InvoiceNum: 1005, Amount: 88, ShipTo: {Street: ""3 Main St."", City: ""Palo Alto"", State: ""CA"", Zip: ""94303""}}"
//     002,"{Name: ""Bat Masterson"", Id: 2, Active: true, Address: {Street: ""777 Any St."", City: ""Collins"", State: ""IA"", Zip: ""50055""}, Credit: {Agency: TransUnion, Report: TransUnionReport: {Rating: 80}}}"
//     002001,"{CustId: 2, InvoiceNum: 1001, Amount: 166, ShipTo: {Street: ""777 Any St."", City: ""Collins"", State: ""IA"", Zip: ""50055""}}"
//     002002,"{CustId: 2, InvoiceNum: 1002, Amount: 243, ShipTo: {Street: ""888 Any St."", City: ""Collins"", State: ""IA"", Zip: ""50055""}}"
//     002003,"{CustId: 2, InvoiceNum: 1004, Amount: 787, ShipTo: {Street: ""999 Any St."", City: ""Collins"", State: ""IA"", Zip: ""50055""}}"
//     002004,"{CustId: 2, InvoiceNum: 1006, Amount: 88, ShipTo: {Street: ""101010 Any St."", City: ""Collins"", State: ""IA"", Zip: ""50055""}}"
//     ? exit;
//     >
package main
