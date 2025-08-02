Project: Memory Organization Overhaul
Objective: To implement a robust system for organizing memories into categories, similar to Spotify's playlist functionality, and to redesign the UI for a more intuitive user experience.
Part 1: Backend - Data Model (The Foundation)
This architecture will be implemented using a many-to-many relationship managed by a central join table.
1. Category Table
This table stores the details of each user-defined category.
Attribute
Type
Description
categoryId
string / uuid
Primary Key. A unique identifier for the category.
userId
string
Foreign Key. Links to the user who created it.
title
string
The name of the category (e.g., "Work Projects").
description
text (optional)
A short description of the category's purpose.
createdAt
timestamp
The date and time the category was created.

2. Memory Table
This table stores individual memories. It is simplified to manage its own data, plus the new tags attribute.
Attribute
Type
Description
memoryId
string / uuid
Primary Key. A unique identifier for the memory.
userId
string
Foreign Key. Links to the user who created it.
title
string
The title of the memory.
content
text
The main body of the memory.
tags
array of strings
A list of simple string tags (e.g., ["urgent"]). Defaults to [].
createdAt
timestamp
The date and time the memory was created.

3. CategoryMemory Join Table
This crucial link table connects Categories and Memories.
Attribute
Type
Description
id
string / uuid
Primary Key. A unique identifier for this specific link.
categoryId
string
Foreign Key. Points to Category.categoryId.
memoryId
string
Foreign Key. Points to Memory.memoryId.
addedAt
timestamp
The date and time the memory was added to the category.

Part 2: Frontend - UI/UX Redesign (The User Experience)
1. Main Navigation Relocation
Action: Move existing navigation links (Profile, Settings, Analytics, Help) from the left sidebar.
New Location: Place them inside a dropdown menu accessible from the user's profile avatar/name in the top-right header.
2. Left Sidebar Revamp
Action: Dedicate the left sidebar exclusively to category navigation.
Content:
"All Categories" Button: At the top of the sidebar, add a button linking to the /categories page.
Quick Access List: Below the button, display a list of the user's categories for direct navigation to a category's detail page (/categories/{id}).
3. New Page: "All Categories" View
Route: /categories
Layout: A grid or list of "Category Cards."
Card Content: Each card must display its title and description.
Functionality: Clicking a card navigates to the corresponding "Category Detail" view.
4. New Page: "Category Detail" View
Route: /categories/{categoryId}
Layout:
Header: Prominently display the title and description of the category.
Content: A list of all memories belonging to this category.
Functionality: Clicking a memory navigates to the standard view/edit page for that memory.
Part 3: Implementation Plan (The Roadmap)
Phase 1: Backend Setup
Database Migration:
Create migrations to introduce the Category, Memory (if modifying), and CategoryMemory tables/schemas.
API for Categories (CRUD):
POST /api/categories - Create a new category.
GET /api/categories - Get all categories for the logged-in user.
GET /api/categories/{id} - Get a single category's details.
PUT /api/categories/{id} - Update a category.
DELETE /api/categories/{id} - Delete a category.
API for Managing Memories in Categories:
POST /api/categories/{id}/memories - Add an existing memory to a category (creates a CategoryMemory entry).
GET /api/categories/{id}/memories - Get all memories within a category.
DELETE /api/categories/{categoryId}/memories/{memoryId} - Remove a memory from a category (deletes a CategoryMemory entry).
Update Memory API:
Modify POST /api/memories and PUT /api/memories/{id} to handle the new tags array.
Phase 2: Frontend Development
Refactor Navigation: Move the main navigation links into the profile dropdown menu first.
Build Category Sidebar: Implement the new sidebar, fetching data from GET /api/categories to populate the quick access list.
Build "All Categories" Page: Create the /categories page, fetch data, and render the category cards.
Build "Category Detail" Page: Create the dynamic /categories/{id} page. On load, fetch and display category details and the list of associated memories.
Integrate "Add to Category" Feature: In the UI for a single memory, add a button/dropdown to add that memory to one or more categories, calling the appropriate API endpoint.
