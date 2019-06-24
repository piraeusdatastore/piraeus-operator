/*
* A REST client to interact with LINSTOR's REST API
* Copyright © 2019 LINBIT HA-Solutions GmbH
* Author: Roland Kammerer <roland.kammerer@linbit.com>
*
* This program is free software; you can redistribute it and/or modify
* it under the terms of the GNU General Public License as published by
* the Free Software Foundation; either version 2 of the License, or
* (at your option) any later version.
*
* This program is distributed in the hope that it will be useful,
* but WITHOUT ANY WARRANTY; without even the implied warranty of
* MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
* GNU General Public License for more details.
*
* You should have received a copy of the GNU General Public License
* along with this program; if not, see <http://www.gnu.org/licenses/>.
 */

package client

import "context"

// custom code

// EncryptionService is the service that deals with encyrption related tasks.
type EncryptionService struct {
	client *Client
}

// Passphrase represents a LINSTOR passphrase
type Passphrase struct {
	NewPassphrase string `json:"new_passphrase,omitempty"`
	OldPassphrase string `json:"old_passphrase,omitempty"`
}

// Create creates an encryption with the given passphrase
func (n *EncryptionService) Create(ctx context.Context, passphrase Passphrase) error {
	_, err := n.client.doPOST(ctx, "/v1/encryption/passphrase", passphrase)
	return err
}

// Modify modifies an existing passphrase
func (n *EncryptionService) Modify(ctx context.Context, passphrase Passphrase) error {
	_, err := n.client.doPUT(ctx, "/v1/encryption/passphrase", passphrase)
	return err
}

// Enter is used to enter a password so that content can be decrypted
func (n *EncryptionService) Enter(ctx context.Context, password string) error {
	_, err := n.client.doPATCH(ctx, "/v1/encryption/passphrase", password)
	return err
}
