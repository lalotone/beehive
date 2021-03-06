/*
 *    Copyright (C) 2014 Christian Muehlhaeuser
 *
 *    This program is free software: you can redistribute it and/or modify
 *    it under the terms of the GNU Affero General Public License as published
 *    by the Free Software Foundation, either version 3 of the License, or
 *    (at your option) any later version.
 *
 *    This program is distributed in the hope that it will be useful,
 *    but WITHOUT ANY WARRANTY; without even the implied warranty of
 *    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *    GNU Affero General Public License for more details.
 *
 *    You should have received a copy of the GNU Affero General Public License
 *    along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 *    Authors:
 *      Christian Muehlhaeuser <muesli@gmail.com>
 */

package bees

import (
	"bytes"
	"log"
	"strings"
	"text/template"

	"github.com/muesli/beehive/filters"
)

// An element in a Chain
type ChainElement struct {
	Action Action
	Filter Filter
}

// A user defined Chain
type Chain struct {
	Name        string
	Description string
	Event       *Event
	Elements    []ChainElement
}

// Execute a filter. Returns whether the filter passed or not.
func execFilter(filter Filter, opts map[string]interface{}) bool {
	f := *filters.GetFilter(filter.Name)
	log.Println("\tExecuting filter:", f.Name(), "-", f.Description())

	for _, opt := range filter.Options {
		log.Println("\t\tOptions:", opt)
		origVal := opts[opt.Name]
		cleanVal := opt.Value
		if opt.Trimmed {
			switch v := origVal.(type) {
			case string:
				origVal = strings.TrimSpace(v)
			}
			switch v := cleanVal.(type) {
			case string:
				cleanVal = strings.TrimSpace(v)
			}
		}
		if opt.CaseInsensitive {
			switch v := origVal.(type) {
			case string:
				origVal = strings.ToLower(v)
			}
			switch v := cleanVal.(type) {
			case string:
				cleanVal = strings.ToLower(v)
			}
		}

		// if value is an array, iterate over it and pass if any of its values pass
		passes := false
		switch v := cleanVal.(type) {
		case []interface{}:
			for _, vi := range v {
				if f.Passes(origVal, vi) {
					passes = true
					break
				}
			}

		default:
			passes = f.Passes(origVal, cleanVal)
		}
		if passes == opt.Inverse {
			return false
		}
	}

	return true
}

// Execute an action and map its ins & outs.
func execAction(action Action, opts map[string]interface{}) bool {
	a := Action{
		Bee:  action.Bee,
		Name: action.Name,
	}

	for _, opt := range action.Options {
		ph := Placeholder{
			Name: opt.Name,
		}

		switch opt.Value.(type) {
		case string:
			var value bytes.Buffer

			funcMap := template.FuncMap{
				"Left": func(values ...interface{}) string {
					return values[0].(string)[:values[1].(int)]
				},
				"Mid": func(values ...interface{}) string {
					if len(values) > 2 {
						return values[0].(string)[values[1].(int):values[2].(int)]
					} else {
						return values[0].(string)[values[1].(int):]
					}
				},
				"Right": func(values ...interface{}) string {
					return values[0].(string)[len(values[0].(string))-values[1].(int):]
				},
				"Split": strings.Split,
				"Last": func(values ...interface{}) string {
					return values[0].([]string)[len(values[0].([]string))-1]
				},
			}

			tmpl, err := template.New(action.Bee + "_" + action.Name + "_" + opt.Name).Funcs(funcMap).Parse(opt.Value.(string))
			if err == nil {
				err = tmpl.Execute(&value, opts)
			}
			if err != nil {
				panic(err)
			}

			ph.Type = "string"
			ph.Value = value.String()

		default:
			ph.Type = opt.Type
			ph.Value = opt.Value
		}
		a.Options = append(a.Options, ph)
	}

	log.Println("\tExecuting action:", a.Bee, "/", a.Name, "-", GetActionDescriptor(&a).Description)
	for _, v := range a.Options {
		log.Println("\t\tOptions:", v)
	}
	(*GetBee(a.Bee)).Action(a)

	return true
}

// Execute chains for an event we received.
func execChains(event *Event) {
	for _, c := range chains {
		if c.Event.Name != event.Name || c.Event.Bee != event.Bee {
			continue
		}

		log.Println("Executing chain:", c.Name, "-", c.Description)
		for _, el := range c.Elements {
			m := make(map[string]interface{})
			for _, opt := range event.Options {
				m[opt.Name] = opt.Value
			}

			if el.Filter.Name != "" {
				if execFilter(el.Filter, m) {
					log.Println("\t\tPassed filter!")
				} else {
					log.Println("\t\tDid not pass filter!")
					break
				}
			}
			if el.Action.Name != "" {
				execAction(el.Action, m)
			}
		}
	}
}
