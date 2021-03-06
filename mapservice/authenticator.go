// vi:set ai sm nu ts=4 sw=4 fileencoding=utf-8:
/*
########################################################################################
#  _______  _______  _______                ___       _______     _______              #
# (  ____ \(       )(  ___  )              /   )     / ___   )   / ___   )             #
# | (    \/| () () || (   ) |             / /) |     \/   )  |   \/   )  |             #
# | |      | || || || (___) |            / (_) (_        /   )       /   )             #
# | | ____ | |(_)| ||  ___  |           (____   _)     _/   /      _/   /              #
# | | \_  )| |   | || (   ) | Game           ) (      /   _/      /   _/               #
# | (___) || )   ( || )   ( | Master's       | |   _ (   (__/\ _ (   (__/\             #
# (_______)|/     \||/     \| Assistant      (_)  (_)\_______/(_)\_______/             #
#                                                                                      #
########################################################################################
*/
//
////////////////////////////////////////////////////////////////////////////////////////
//                                                                                    //
//                                  Authenticator                                     //
//                                                                                    //
// Calculates the challenge/response for a login attempt, and checks user input to    //
// determine if it was a valid response to the challenge.                             //
//                                                                                    //
// The credential may be a shared player password for the game group, or one reserved //
// for the GM's access, or an individual user may have set a personal password to use //
// for their own sessions. By the time we get to this code, we have two passwords in  //
// play: one that is either the shared password or (if one is set) the one for the    //
// user attempting to log in, and the other is the GM's password set in the server    //
// configuration. The response from the client is checked against both passwords      // 
// since using the GM password indicates GM regardless of login name (which may come  //
// from the username on the client OS).                                               //
//                                                                                    //
////////////////////////////////////////////////////////////////////////////////////////

package mapservice

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
)

//////////////////////////////////////////////////////////////////////////////////
//     _         _   _                _   _           _             
//    / \  _   _| |_| |__   ___ _ __ | |_(_) ___ __ _| |_ ___  _ __ 
//   / _ \| | | | __| '_ \ / _ \ '_ \| __| |/ __/ _` | __/ _ \| '__|
//  / ___ \ |_| | |_| | | |  __/ | | | |_| | (_| (_| | || (_) | |   
// /_/   \_\__,_|\__|_| |_|\___|_| |_|\__|_|\___\__,_|\__\___/|_|   
//                                                                  
// An Authenticator object holds the authentication challenge/response
// context for a single client transaction.

type Authenticator struct {
	Secret	  []byte	// unprivileged (player) authentication password
	GmSecret  []byte	// privileged (GM) authentication password
	Challenge []byte	// current generated challenge or nil
	Client    string	// text description of client program/version
	Username  string	// name of user authenticating
	GmMode    bool		// true if GM authentication succeeded
}

//
// Change the secret (for when we know the username
// they are logging in as and that user has their own
// password). This disables GM logins.
//
func (a *Authenticator) SetSecret(secret []byte) {
	a.GmSecret = []byte{}
	a.GmMode = false
	a.Secret = secret
}

//
// Compare two byte arrays for equality.
//
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) { return false }
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

//
// Generate an authentication challenge for this session.
// This is a 256-bit random nonce which is remembered for
// subsequent operations on this Authenticator instance.
//
// The challenge is returned as a base-64-encoded string.
//
func (a *Authenticator) GenerateChallenge() (string, error) {
	a.Challenge = make([]byte, 32)
	a.GmMode = false
	_, err := rand.Read(a.Challenge)
	if err != nil { return "", err }

	// The first two bytes of the nonce determine the number of
	// rounds of hashing that are done for the response. We'll
	// limit the number to something in the range [64,4095] to
	// prevent a trivial run or causing a performance issue on
	// the client by requiring too many.
	a.Challenge[0] &= 0x0f
	a.Challenge[1] |= 0x40

	return a.CurrentChallenge(), nil
}

//
// Return the last-generated challenge created by GenerateChallenge().
// This is returned as a base-64-encoded string suitable for transmitting
// directly to a client.
//
func (a *Authenticator) CurrentChallenge() (string) {
	return base64.StdEncoding.EncodeToString(a.Challenge)
}

//
// Calculate the correct response for the previously generated challenge
// value (which is stored in the Authenticator instance on which this
// method is called).
//
// The response is:
//    R = H(S || H(S || ... H(C||S)))
//        \________________/
//                 P
//
// Where C = the generated 256-bit binary challenge value
//       S = the user's secret
//    H(x) = the binary SHA-256 hash of x
//       P = the randomly-chosen number of rounds through H(x)
//
// P is a random value from 0-65535 which is obtained by taking the
// first two bytes of C as a 16-bit big-endian binary integer value.
// (The current server will ask for something between 64 and 4095 rounds,
// but the code here will handle the entire unsigned 16-bit range.)
//
// **NOTE** This is not the most robust of challenge-response implementations,
// but deemed adequate for our purposes at the moment. Specifically, it is
// vulnerable to MitM attacks if the communications path is not otherwise 
// protected, and long-term observation of authentication attempts may
// provide sufficient insight to determine the secret component of the
// exchange if the communcation channel is not otherwise encrypted.
//
// I wouldn't normally say this, but in this very specific case, what we
// are protecting is so very, very unimportant to anyone, and the result
// of obtaining a credential is to either watch us play D&D or interfere
// with moving our pieces on the board, so we're ok with this simple
// approach for the time being. We'll most likely upgrade this to have
// additional protections in the future.
//
func (a *Authenticator) calcResponse(secret []byte) ([]byte, error) {
	var i uint16

	if len(a.Challenge) < 8 {
		return nil, fmt.Errorf("No (or insufficient) challenge value set; unable to determine response")
	}
	if len(secret) == 0 {
		return nil, fmt.Errorf("No secret value set; unable to determine response")
	}

	passes := binary.BigEndian.Uint16(a.Challenge[0:2])
	h := sha256.New()
	h.Write(a.Challenge)
	h.Write(secret)
	d := h.Sum(nil)
	for i = 0; i < passes; i++ {
		h.Reset()
		h.Write(secret)
		h.Write(d)
		d = h.Sum(nil)
	}

	return d, nil
}

//
// Given a base-64-endoded response string, verify that the value it encodes
// matches the expected response for the previously-generated challenge.
//
// If the response is not correct for the expected user's secret, we try
// again against the GM's secret to see if the user is logging in as the GM
// role.
//
func (a *Authenticator) ValidateResponse(response string) (bool, error) {
	if len(a.Secret) == 0 {
		return false, fmt.Errorf("No password configured")
	}
	binary_response, err := base64.StdEncoding.DecodeString(response)
	if err != nil {
		return false, fmt.Errorf("Error decoding client response: %v", err)
	}
	our_response, err := a.calcResponse(a.Secret)
	if err != nil {
		return false, fmt.Errorf("Error validating client response: %v", err)
	}
	if bytesEqual(our_response, binary_response) {
		return true, nil
	}
	if len(a.GmSecret) > 0 {
		our_response, err := a.calcResponse(a.GmSecret)
		if err != nil {
			return false, fmt.Errorf("Error validating client response (gm): %v", err)
		}
		if bytesEqual(our_response, binary_response) {
			a.GmMode = true
			return true, nil
		}
	}
	return false, nil
}

//
// Reset an Authenticator instance back to its uninitialized state
// so that it may be used again for another authentication attempt.
//
func (a *Authenticator) Reset() {
	a.Challenge = []byte{}
	a.GmMode = false
}
// @[00]@| GMA 4.2.2
// @[01]@|
// @[10]@| Copyright © 1992–2020 by Steven L. Willoughby
// @[11]@| (AKA Software Alchemy), Aloha, Oregon, USA. All Rights Reserved.
// @[12]@| Distributed under the terms and conditions of the BSD-3-Clause
// @[13]@| License as described in the accompanying LICENSE file distributed
// @[14]@| with GMA.
// @[15]@|
// @[20]@| Redistribution and use in source and binary forms, with or without
// @[21]@| modification, are permitted provided that the following conditions
// @[22]@| are met:
// @[23]@| 1. Redistributions of source code must retain the above copyright
// @[24]@|    notice, this list of conditions and the following disclaimer.
// @[25]@| 2. Redistributions in binary form must reproduce the above copy-
// @[26]@|    right notice, this list of conditions and the following dis-
// @[27]@|    claimer in the documentation and/or other materials provided
// @[28]@|    with the distribution.
// @[29]@| 3. Neither the name of the copyright holder nor the names of its
// @[30]@|    contributors may be used to endorse or promote products derived
// @[31]@|    from this software without specific prior written permission.
// @[32]@|
// @[33]@| THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND
// @[34]@| CONTRIBUTORS “AS IS” AND ANY EXPRESS OR IMPLIED WARRANTIES,
// @[35]@| INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
// @[36]@| MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// @[37]@| DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS
// @[38]@| BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY,
// @[39]@| OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// @[40]@| PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
// @[41]@| PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// @[42]@| THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR
// @[43]@| TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF
// @[44]@| THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
// @[45]@| SUCH DAMAGE.
// @[46]@|
// @[50]@| This software is not intended for any use or application in which
// @[51]@| the safety of lives or property would be at risk due to failure or
// @[52]@| defect of the software.
