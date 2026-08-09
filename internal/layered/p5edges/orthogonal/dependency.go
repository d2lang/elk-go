// Copyright (c) 2010, 2020 Kiel University and others.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0.
//
// SPDX-License-Identifier: EPL-2.0

package orthogonal

// DependencyType is ELK 0.8.1's HyperEdgeSegmentDependency.DependencyType.
type DependencyType uint8

const (
	RegularDependency DependencyType = iota
	CriticalDependency
)

const criticalDependencyWeight = 1

// HyperEdgeSegmentDependency says that Source wants a lower routing slot than
// Target. Critical dependencies prevent overlaps; regular dependencies carry
// the crossing/conflict cost of ignoring them.
type HyperEdgeSegmentDependency struct {
	Type   DependencyType
	Source *HyperEdgeSegment
	Target *HyperEdgeSegment
	Weight int
}

func AddRegularDependency(source, target *HyperEdgeSegment, weight int) *HyperEdgeSegmentDependency {
	return addDependency(RegularDependency, source, target, weight)
}

func AddCriticalDependency(source, target *HyperEdgeSegment) *HyperEdgeSegmentDependency {
	return addDependency(CriticalDependency, source, target, criticalDependencyWeight)
}

func addDependency(kind DependencyType, source, target *HyperEdgeSegment, weight int) *HyperEdgeSegmentDependency {
	dependency := &HyperEdgeSegmentDependency{Type: kind, Weight: weight}
	dependency.setSource(source)
	dependency.setTarget(target)
	return dependency
}

func (dependency *HyperEdgeSegmentDependency) Remove() {
	dependency.setSource(nil)
	dependency.setTarget(nil)
}

func (dependency *HyperEdgeSegmentDependency) Reverse() {
	source, target := dependency.Source, dependency.Target
	dependency.setSource(target)
	dependency.setTarget(source)
}

func (dependency *HyperEdgeSegmentDependency) setSource(source *HyperEdgeSegment) {
	if dependency.Source != nil {
		dependency.Source.OutgoingDependencies = removeDependency(dependency.Source.OutgoingDependencies, dependency)
	}
	dependency.Source = source
	if source != nil {
		source.OutgoingDependencies = append(source.OutgoingDependencies, dependency)
	}
}

func (dependency *HyperEdgeSegmentDependency) setTarget(target *HyperEdgeSegment) {
	if dependency.Target != nil {
		dependency.Target.IncomingDependencies = removeDependency(dependency.Target.IncomingDependencies, dependency)
	}
	dependency.Target = target
	if target != nil {
		target.IncomingDependencies = append(target.IncomingDependencies, dependency)
	}
}

func removeDependency(values []*HyperEdgeSegmentDependency, target *HyperEdgeSegmentDependency) []*HyperEdgeSegmentDependency {
	for index, value := range values {
		if value == target {
			copy(values[index:], values[index+1:])
			return values[:len(values)-1]
		}
	}
	return values
}
