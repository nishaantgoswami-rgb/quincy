package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPKCE_CodeVerifierGeneration(t *testing.T) {
	// Test multiple generations to ensure randomness
	verifiers := make(map[string]bool)
	for i := 0; i < 10; i++ {
		verifier, err := generateCodeVerifier()
		require.NoError(t, err)
		assert.NotEmpty(t, verifier)
		
		// Verifier should be at least 43 characters and at most 128 characters
		assert.GreaterOrEqual(t, len(verifier), 43)
		assert.LessOrEqual(t, len(verifier), 128)
		
		// Should not have been generated before (ensuring randomness)
		assert.False(t, verifiers[verifier], "Duplicate verifier generated")
		verifiers[verifier] = true
		
		// Should only contain URL-safe characters
		for _, char := range verifier {
			assert.True(t, (char >= 'A' && char <= 'Z') ||
				(char >= 'a' && char <= 'z') ||
				(char >= '0' && char <= '9') ||
				char == '-' || char == '.' || char == '_' || char == '~',
				"Verifier contains invalid character: %c", char)
		}
	}
}

func TestPKCE_CodeChallengeGeneration(t *testing.T) {
	testCases := []struct {
		verifier  string
		expected  string
		name      string
	}{
		{
			verifier: "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk",
			expected: "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
			name:     "RFC 7636 example",
		},
		{
			verifier: "test-verifier-for-testing-purposes-with-sufficient-length",
			name:     "Custom test verifier",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			challenge := generateCodeChallenge(tc.verifier)
			assert.NotEmpty(t, challenge)
			
			// Challenge should be 43 characters long (base64url encoded SHA256)
			assert.Equal(t, 43, len(challenge))
			
			// Verify the challenge is correctly computed
			hash := sha256.Sum256([]byte(tc.verifier))
			expectedChallenge := base64.RawURLEncoding.EncodeToString(hash[:])
			assert.Equal(t, expectedChallenge, challenge)
			
			// Should only contain URL-safe characters
			for _, char := range challenge {
				assert.True(t, (char >= 'A' && char <= 'Z') ||
					(char >= 'a' && char <= 'z') ||
					(char >= '0' && char <= '9') ||
					char == '-' || char == '_',
					"Challenge contains invalid character: %c", char)
			}
		})
	}
}

func TestPKCE_VerifierChallengeRelationship(t *testing.T) {
	// Test that different verifiers produce different challenges
	challenges := make(map[string]bool)
	for i := 0; i < 5; i++ {
		verifier, err := generateCodeVerifier()
		require.NoError(t, err)
		
		challenge := generateCodeChallenge(verifier)
		assert.NotEmpty(t, challenge)
		
		// Challenge should not have been generated before
		assert.False(t, challenges[challenge], "Duplicate challenge generated")
		challenges[challenge] = true
	}
}

func TestPKCE_CodeVerifierValidation(t *testing.T) {
	// While we don't have explicit validation in our implementation,
	// we can test that our generation produces valid verifiers
	// and that the challenge generation works correctly
	for i := 0; i < 10; i++ {
		verifier, err := generateCodeVerifier()
		require.NoError(t, err)
		
		// Generated verifier should produce a valid challenge
		challenge := generateCodeChallenge(verifier)
		assert.NotEmpty(t, challenge)
		assert.Equal(t, 43, len(challenge))
	}
}