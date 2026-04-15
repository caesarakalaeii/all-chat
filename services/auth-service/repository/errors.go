// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package repository

import "errors"

// ErrUserNotFound is returned when a user lookup fails to find a record.
var ErrUserNotFound = errors.New("user not found")

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// ErrPlatformAlreadyLinked is returned when trying to link a platform identity
// that is already associated with a different viewer account.
var ErrPlatformAlreadyLinked = errors.New("platform already linked to a different account")

// ErrLastPlatform is returned when trying to unlink the last remaining platform
// identity for a viewer. The viewer must always retain at least one connection.
var ErrLastPlatform = errors.New("cannot unlink the last platform — viewer must have at least one connection")
