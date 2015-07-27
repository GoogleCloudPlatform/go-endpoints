// Copyright 2015 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helloworld

import (
	"log"
	"time"

	"github.com/GoogleCloudPlatform/go-endpoints/endpoints"

	"appengine/datastore"
)

// Greeting is a datastore entity that represents a single greeting.
// It also serves as (a part of) a response of GreetingService.
type Greeting struct {
	Key     *datastore.Key `json:"id" datastore:"-"`
	Author  string         `json:"author"`
	Content string         `json:"content" datastore:",noindex"`
	Date    time.Time      `json:"date"`
}

// GretingAddReq is the request type for GreetingService.Add.
type GreetingAddReq struct {
	Author  string `json:"author"`
	Content string `json:"content" endpoints:"req"`
}

// GreetingListReq is the request type for GreetingService.List.
type GreetingsListReq struct {
	Limit int `json:"limit" endpoints:"d=10"`
}

// GreetingsList is the response type for GreetingService.List.
type GreetingsList struct {
	Items []*Greeting `json:"items"`
}

// GreetingService offers operations to add and list greetings.
type GreetingService struct{}

// List responds with a list of all greetings ordered by Date field.
// Most recent greets come first.
func (gs *GreetingService) List(c endpoints.Context, r *GreetingsListReq) (*GreetingsList, error) {
	q := datastore.NewQuery("Greeting").Order("-Date").Limit(r.Limit)
	greets := make([]*Greeting, 0, r.Limit)
	keys, err := q.GetAll(c, &greets)
	if err != nil {
		return nil, err
	}

	for i, k := range keys {
		greets[i].Key = k
	}
	return &GreetingsList{greets}, nil
}

// Add adds a greeting.
func (gs *GreetingService) Add(c endpoints.Context, r *GreetingAddReq) error {
	k := datastore.NewIncompleteKey(c, "Greeting", nil)
	g := &Greeting{
		Author:  r.Author,
		Content: r.Content,
		Date:    time.Now(),
	}
	_, err := datastore.Put(c, k, g)
	return err
}

func init() {
	api, err := endpoints.RegisterService(&GreetingService{},
		"greetings", "v1", "Greetings API", true)
	if err != nil {
		log.Fatalf("Register service: %v", err)
	}

	list := api.MethodByName("List").Info()
	list.HTTPMethod = "GET"
	list.Path = "greetings"
	list.Name = "list"
	list.Desc = "List most recent greetings."

	add := api.MethodByName("Add").Info()
	add.HTTPMethod = "PUT"
	add.Path = "greetings"
	add.Name = "add"
	add.Desc = "Add a greeting."

	endpoints.HandleHTTP()
}
