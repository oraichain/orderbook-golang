// Copyright (c) 2019, Agiletech Viet Nam. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// References: http://en.wikipedia.org/wiki/Red%E2%80%93black_tree
package orderbook

import (
	"bytes"
	"fmt"
)

// Tree holds elements of the red-black tree
type Tree struct {
	db          *BatchDatabase
	rootKey     []byte
	size        uint64
	Comparator  Comparator
	FormatBytes FormatBytes
}

// NewWith instantiates a red-black tree with the custom comparator.
func NewWith(comparator Comparator, db *BatchDatabase) *Tree {

	tree := &Tree{
		Comparator: comparator,
		db:         db,
	}
	return tree
}

func NewWithBytesComparator(db *BatchDatabase) *Tree {
	return NewWith(
		bytes.Compare,
		db,
	)
}

func (tree *Tree) Root() *Node {
	root, _ := tree.GetNode(tree.rootKey)
	return root
}

func (tree *Tree) IsEmptyKey(key []byte) bool {
	return tree.db.IsEmptyKey(key)
}

func (tree *Tree) SetRootKey(key []byte, size uint64) {
	tree.rootKey = key
	tree.size = size
}

// Put inserts node into the tree.
// Key should adhere to the comparator's type assertion, otherwise method panics.
func (tree *Tree) Put(key []byte, value []byte) error {
	var insertedNode *Node
	if tree.IsEmptyKey(tree.rootKey) {
		// Assert key is of comparator's type for initial tree
		item := &Item{Value: value, Color: red, Keys: &KeyMeta{}}
		tree.rootKey = key
		insertedNode = &Node{Key: key, Item: item}
	} else {
		node := tree.Root()
		loop := true
		for loop {
			compare := tree.Comparator(key, node.Key)
			// fmt.Printf("Comparing :%v\n", compare)
			switch {
			case compare == 0:
				// fmt.Printf("UPDATE CONTENT ONLY :%v\n", compare)
				node.Item.Value = value
				tree.Save(node)
				return nil
			case compare < 0:
				if tree.IsEmptyKey(node.LeftKey()) {
					node.LeftKey(key)
					tree.Save(node)
					item := &Item{Value: value, Color: red, Keys: &KeyMeta{}}
					nodeLeft := &Node{Key: key, Item: item}
					insertedNode = nodeLeft
					loop = false
				} else {
					node = node.Left(tree)
				}
			case compare > 0:

				if tree.IsEmptyKey(node.RightKey()) {
					node.RightKey(key)
					tree.Save(node)
					item := &Item{Value: value, Color: red, Keys: &KeyMeta{}}
					nodeRight := &Node{Key: key, Item: item}
					insertedNode = nodeRight
					loop = false
				} else {
					// fmt.Printf("Noderight :%s:%s\n", node.RightKey(), key)
					node = node.Right(tree)
				}

			}
		}

		insertedNode.ParentKey(node.Key)
		tree.Save(insertedNode)

	}

	tree.insertCase1(insertedNode)
	tree.Save(insertedNode)

	// fmt.Println(tree)
	tree.size++
	return nil
}

func (tree *Tree) GetNode(key []byte) (*Node, error) {

	item := &Item{}

	val, err := tree.db.Get(key, item)

	if err != nil || val == nil {
		return nil, err
	}

	return &Node{Key: key, Item: val.(*Item)}, err
}

func (tree *Tree) Has(key []byte) (bool, error) {
	return tree.db.Has(key)
}

// Get searches the node in the tree by key and returns its value or nil if key is not found in tree.
// Second return parameter is true if key was found, otherwise false.
// Key should adhere to the comparator's type assertion, otherwise method panics.
func (tree *Tree) Get(key []byte) (value []byte, found bool) {

	node, err := tree.GetNode(key)
	if err != nil {
		return nil, false
	}
	if node != nil {
		return node.Item.Value, true
	}
	return nil, false
}

// Remove remove the node from the tree by key.
// Key should adhere to the comparator's type assertion, otherwise method panics.
func (tree *Tree) Remove(key []byte) {
	var child *Node
	node, err := tree.GetNode(key)

	if err != nil || node == nil {
		return
	}

	var left, right *Node = nil, nil
	if !tree.IsEmptyKey(node.LeftKey()) {
		left = node.Left(tree)
	}
	if !tree.IsEmptyKey(node.RightKey()) {
		right = node.Right(tree)
	}

	if left != nil && right != nil {
		node = left.maximumNode(tree)
	}

	if left == nil || right == nil {
		if right == nil {
			child = left
		} else {
			child = right
		}

		if node.Item.Color {
			node.Item.Color = nodeColor(child)
			tree.Save(node)

			tree.deleteCase1(node)
		}

		tree.replaceNode(node, child)

		if tree.IsEmptyKey(node.ParentKey()) && child != nil {
			child.Item.Color = black
			tree.Save(child)
		}
	}

	tree.size--
}

// // Empty returns true if tree does not contain any nodes
func (tree *Tree) Empty() bool {
	return tree.size == 0
}

// Size returns number of nodes in the tree.
func (tree *Tree) Size() uint64 {
	return tree.size
}

// Keys returns all keys in-order
func (tree *Tree) Keys() [][]byte {
	keys := make([][]byte, tree.size)
	it := tree.Iterator()
	for i := 0; it.Next(); i++ {
		keys[i] = it.Key()
	}
	return keys
}

// Values returns all values in-order based on the key.
func (tree *Tree) Values() [][]byte {
	values := make([][]byte, tree.size)
	it := tree.Iterator()
	for i := 0; it.Next(); i++ {
		values[i] = it.Value()
	}
	return values
}

// Left returns the left-most (min) node or nil if tree is empty.
func (tree *Tree) Left() *Node {
	var parent *Node
	current := tree.Root()
	for current != nil {
		parent = current
		current = current.Left(tree)
	}
	return parent
}

// Right returns the right-most (max) node or nil if tree is empty.
func (tree *Tree) Right() *Node {
	var parent *Node
	current := tree.Root()
	for current != nil {
		parent = current
		current = current.Right(tree)
	}
	return parent
}

// Floor Finds floor node of the input key, return the floor node or nil if no floor is found.
// Second return parameter is true if floor was found, otherwise false.
//
// Floor node is defined as the largest node that is smaller than or equal to the given node.
// A floor node may not be found, either because the tree is empty, or because
// all nodes in the tree are larger than the given node.
//
// Key should adhere to the comparator's type assertion, otherwise method panics.
func (tree *Tree) Floor(key []byte) (floor *Node, found bool) {
	found = false
	node := tree.Root()
	for node != nil {
		compare := tree.Comparator(key, node.Key)
		switch {
		case compare == 0:
			return node, true
		case compare < 0:
			node = node.Left(tree)
		case compare > 0:
			floor, found = node, true
			node = node.Right(tree)
		}
	}
	if found {
		return floor, true
	}
	return nil, false
}

// Ceiling finds ceiling node of the input key, return the ceiling node or nil if no ceiling is found.
// Second return parameter is true if ceiling was found, otherwise false.
//
// Ceiling node is defined as the smallest node that is larger than or equal to the given node.
// A ceiling node may not be found, either because the tree is empty, or because
// all nodes in the tree are smaller than the given node.
//
// Key should adhere to the comparator's type assertion, otherwise method panics.
func (tree *Tree) Ceiling(key []byte) (ceiling *Node, found bool) {
	found = false
	node := tree.Root()
	for node != nil {
		compare := tree.Comparator(key, node.Key)
		switch {
		case compare == 0:
			return node, true
		case compare < 0:
			ceiling, found = node, true
			node = node.Left(tree)
		case compare > 0:
			node = node.Right(tree)
		}
	}
	if found {
		return ceiling, true
	}
	return nil, false
}

// Clear removes all nodes from the tree.
// we do not delete other children, but update them by overriding later
func (tree *Tree) Clear() {
	tree.rootKey = EmptyKey()
	tree.size = 0
}

// String returns a string representation of container
func (tree *Tree) String() string {
	str := fmt.Sprintf("RedBlackTree, size: %d\n", tree.size)

	// if !tree.Empty() {
	output(tree, tree.Root(), "", true, &str)
	// }
	return str
}

func output(tree *Tree, node *Node, prefix string, isTail bool, str *string) {
	// fmt.Printf("Node : %v+\n", node)
	if node == nil {
		return
	}

	if !tree.IsEmptyKey(node.RightKey()) {
		newPrefix := prefix
		if isTail {
			newPrefix += "│   "
		} else {
			newPrefix += "    "
		}
		output(tree, node.Right(tree), newPrefix, false, str)
	}
	*str += prefix
	if isTail {
		*str += "└── "
	} else {
		*str += "┌── "
	}

	if tree.FormatBytes != nil {
		*str += node.String(tree) + "\n"
	} else {
		*str += string(node.Key) + "\n"
	}

	if !tree.IsEmptyKey(node.LeftKey()) {
		newPrefix := prefix
		if isTail {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}
		output(tree, node.Left(tree), newPrefix, true, str)
	}
}

func (tree *Tree) rotateLeft(node *Node) {
	right := node.Right(tree)
	tree.replaceNode(node, right)
	node.RightKey(right.LeftKey())
	if !tree.IsEmptyKey(right.LeftKey()) {
		rightLeft := right.Left(tree)
		rightLeft.ParentKey(node.Key)
		tree.Save(rightLeft)
	}
	right.LeftKey(node.Key)
	node.ParentKey(right.Key)
	tree.Save(node)
	tree.Save(right)
}

func (tree *Tree) rotateRight(node *Node) {
	left := node.Left(tree)
	tree.replaceNode(node, left)
	node.LeftKey(left.RightKey())
	if !tree.IsEmptyKey(left.RightKey()) {
		leftRight := left.Right(tree)
		leftRight.ParentKey(node.Key)
		tree.Save(leftRight)
	}
	left.RightKey(node.Key)
	node.ParentKey(left.Key)
	tree.Save(node)
	tree.Save(left)
}

func (tree *Tree) replaceNode(old *Node, new *Node) {

	// we do not change any byte of Key so we can copy the reference to save directly to db
	var newKey []byte
	if new == nil {
		newKey = EmptyKey()
	} else {
		newKey = new.Key
	}

	if tree.IsEmptyKey(old.ParentKey()) {
		// tree.Root = new
		tree.rootKey = newKey
	} else {
		// update left and right for oldParent
		oldParent := old.Parent(tree)
		// if new != nil {

		// fmt.Printf("Update new %v\n", new)
		// fmt.Printf("Update old parent %v\n", oldParent)
		if tree.Comparator(old.Key, oldParent.LeftKey()) == 0 {
			oldParent.LeftKey(newKey)
		} else {
			// remove oldParent right
			oldParent.RightKey(newKey)
		}
		// fmt.Printf("Update old parent %v\n", oldParent)
		// we can have case like: remove a node, then add it again
		tree.Save(oldParent)
		// }
		// fmt.Println("Replace tree node", old, new, oldParent)
	}
	if new != nil {
		// here is the swap, not update key
		// new.Parent = old.Parent
		new.ParentKey(old.ParentKey())
		tree.Save(new)
	}

	// fmt.Println("Final tree", tree)

}

func (tree *Tree) insertCase1(node *Node) {

	// fmt.Printf("Insert case1 :%s\n", node)
	if tree.IsEmptyKey(node.ParentKey()) {
		node.Item.Color = black
		// store this
		// tree.Save(node)
		// fmt.Println("Breaking case1")
	} else {
		tree.insertCase2(node)
	}
}

func (tree *Tree) insertCase2(node *Node) {
	parent := node.Parent(tree)
	// fmt.Printf("Insert case 2, parent: %s", parent)
	if nodeColor(parent) {
		// tree.Save(node)
		// fmt.Println("Breaking case2")
		return
	}

	tree.insertCase3(node)
}

func (tree *Tree) insertCase3(node *Node) {
	parent := node.Parent(tree)
	uncle := node.uncle(tree)
	// grandparent := node.grandparent(tree)

	// fmt.Println("grand parent 3", grandparent)
	// fmt.Printf("Insert case 3, uncle: %s\n", uncle)
	if !nodeColor(uncle) {
		parent.Item.Color = black
		uncle.Item.Color = black
		tree.Save(uncle)
		tree.Save(parent)
		grandparent := parent.Parent(tree)
		tree.assertNotNull(grandparent, "grant parent")

		grandparent.Item.Color = red
		tree.insertCase1(grandparent)
		tree.Save(grandparent)
	} else {
		tree.insertCase4(node)
	}
}

func (tree *Tree) insertCase4(node *Node) {
	parent := node.Parent(tree)
	// grandparent := node.grandparent(tree)
	grandparent := parent.Parent(tree)
	tree.assertNotNull(grandparent, "grant parent")
	// fmt.Println("grand parent 4", grandparent)
	if tree.Comparator(node.Key, parent.RightKey()) == 0 &&
		tree.Comparator(parent.Key, grandparent.LeftKey()) == 0 {
		tree.rotateLeft(parent)
		node = node.Left(tree)
	} else if tree.Comparator(node.Key, parent.LeftKey()) == 0 &&
		tree.Comparator(parent.Key, grandparent.RightKey()) == 0 {
		tree.rotateRight(parent)
		node = node.Right(tree)
	}

	tree.insertCase5(node)
}

func (tree *Tree) assertNotNull(node *Node, name string) {
	if node == nil {
		panic(fmt.Sprintf("%s is nil\n", name))
	}
}

func (tree *Tree) insertCase5(node *Node) {
	parent := node.Parent(tree)
	// grandparent := node.grandparent(tree)
	parent.Item.Color = black
	tree.Save(parent)

	grandparent := parent.Parent(tree)
	tree.assertNotNull(grandparent, "grant parent")
	// fmt.Println("grand parent 5", grandparent)
	grandparent.Item.Color = red
	tree.Save(grandparent)
	// fmt.Printf("insertCase5 :%s | %s | %s | %s \n", node.Key, parent.LeftKey(), parent, grandparent.LeftKey())
	// fmt.Printf("insertCase5 :%s | %s \n", parent.RightKey(), grandparent.Right(tree))

	if tree.Comparator(node.Key, parent.LeftKey()) == 0 &&
		tree.Comparator(parent.Key, grandparent.LeftKey()) == 0 {
		tree.rotateRight(grandparent)
	} else if tree.Comparator(node.Key, parent.RightKey()) == 0 &&
		tree.Comparator(parent.Key, grandparent.RightKey()) == 0 {
		tree.rotateLeft(grandparent)
	}

}

func (tree *Tree) Save(node *Node) error {
	// value, err := json.Marshal(node.Item)
	// tree.assertNotNull(node, hex.EncodeToString(node.Key))

	return tree.db.Put(node.Key, node.Item)

}

func (tree *Tree) Commit() error {
	return tree.db.Commit()

}

func (tree *Tree) deleteCase1(node *Node) {
	if tree.IsEmptyKey(node.ParentKey()) {
		tree.deleteNode(node, false)
		return
	}

	tree.deleteCase2(node)
}

func (tree *Tree) deleteCase2(node *Node) {
	parent := node.Parent(tree)
	sibling := node.sibling(tree)

	if !nodeColor(sibling) {
		parent.Item.Color = red
		sibling.Item.Color = black
		tree.Save(parent)
		tree.Save(sibling)
		if tree.Comparator(node.Key, parent.LeftKey()) == 0 {
			tree.rotateLeft(parent)
		} else {
			tree.rotateRight(parent)
		}
	}

	tree.deleteCase3(node)
}

func (tree *Tree) deleteCase3(node *Node) {

	parent := node.Parent(tree)
	sibling := node.sibling(tree)
	siblingLeft := sibling.Left(tree)
	siblingRight := sibling.Right(tree)

	if nodeColor(parent) &&
		nodeColor(sibling) &&
		nodeColor(siblingLeft) &&
		nodeColor(siblingRight) {
		sibling.Item.Color = red
		tree.Save(sibling)
		tree.deleteCase1(parent)

		if tree.db.Debug {
			fmt.Printf("delete node,  key: %x, parentKey :%x\n", node.Key, parent.Key)
		}
		tree.deleteNode(node, false)

	} else {
		tree.deleteCase4(node)
	}

}

func (tree *Tree) deleteCase4(node *Node) {
	parent := node.Parent(tree)
	sibling := node.sibling(tree)
	siblingLeft := sibling.Left(tree)
	siblingRight := sibling.Right(tree)

	if !nodeColor(parent) &&
		nodeColor(sibling) &&
		nodeColor(siblingLeft) &&
		nodeColor(siblingRight) {
		sibling.Item.Color = red
		parent.Item.Color = black
		tree.Save(sibling)
		tree.Save(parent)
	} else {
		tree.deleteCase5(node)
	}
}

func (tree *Tree) deleteCase5(node *Node) {
	parent := node.Parent(tree)
	sibling := node.sibling(tree)
	siblingLeft := sibling.Left(tree)
	siblingRight := sibling.Right(tree)

	if tree.Comparator(node.Key, parent.LeftKey()) == 0 &&
		nodeColor(sibling) &&
		!nodeColor(siblingLeft) &&
		nodeColor(siblingRight) {
		sibling.Item.Color = red
		siblingLeft.Item.Color = black

		tree.Save(sibling)
		tree.Save(siblingLeft)

		tree.rotateRight(sibling)

	} else if tree.Comparator(node.Key, parent.RightKey()) == 0 &&
		nodeColor(sibling) &&
		!nodeColor(siblingRight) &&
		nodeColor(siblingLeft) {
		sibling.Item.Color = red
		siblingRight.Item.Color = black

		tree.Save(sibling)
		tree.Save(siblingRight)

		tree.rotateLeft(sibling)

	}

	tree.deleteCase6(node)
}

func (tree *Tree) deleteCase6(node *Node) {
	parent := node.Parent(tree)
	sibling := node.sibling(tree)
	siblingLeft := sibling.Left(tree)
	siblingRight := sibling.Right(tree)

	sibling.Item.Color = nodeColor(parent)
	parent.Item.Color = black

	tree.Save(sibling)
	tree.Save(parent)

	// fmt.Println("before-update ", tree, sibling, parent, siblingLeft, siblingRight)

	if tree.Comparator(node.Key, parent.LeftKey()) == 0 && !nodeColor(siblingRight) {
		siblingRight.Item.Color = black
		tree.Save(siblingRight)
		tree.rotateLeft(parent)
	} else if !nodeColor(siblingLeft) {
		siblingLeft.Item.Color = black
		tree.Save(siblingLeft)
		tree.rotateRight(parent)
	}

	// update the parent meta then delete the current node from db
	tree.deleteNode(node, false)
	// fmt.Println("update ", tree, parent, sibling)
}

func nodeColor(node *Node) bool {
	if node == nil {
		return black
	}
	return node.Item.Color
}

func (tree *Tree) deleteNode(node *Node, force bool) {
	// do not delete if node is root, and at least one child (size >=2)
	if tree.size > 1 && tree.Comparator(node.Key, tree.rootKey) == 0 {
		return
	}
	tree.db.Delete(node.Key, force)
}
