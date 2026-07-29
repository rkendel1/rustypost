package vault

// purposeAllowList enumerates which secret types may legitimately be
// resolved for a given purpose. It exists so future capability-scoped
// access rules have a single enforcement point (resolver.go) rather than
// being duplicated across every caller that resolves a secret.
var purposeAllowList = map[SecretPurpose][]SecretType{
	SecretPurposeGitHubAPI:       {SecretTypeAccessToken, SecretTypeAPIKey},
	SecretPurposeRepositoryClone: {SecretTypeAccessToken, SecretTypePassword},
	SecretPurposeTestExecution:   {SecretTypeAPIKey, SecretTypeAccessToken, SecretTypeCustom},
	SecretPurposeDeployment:      {SecretTypeAPIKey, SecretTypeAccessToken, SecretTypeConnection},
	SecretPurposeIntegrationSync: {SecretTypeAPIKey, SecretTypeAccessToken, SecretTypeRefreshToken},
	SecretPurposeGeneric: {
		SecretTypeAPIKey, SecretTypeAccessToken, SecretTypeRefreshToken,
		SecretTypePassword, SecretTypePrivateKey, SecretTypeConnection, SecretTypeCustom,
	},
}

// PurposeAllowed reports whether a secret of type t may be resolved for the
// given purpose.
func PurposeAllowed(t SecretType, purpose SecretPurpose) bool {
	allowed, ok := purposeAllowList[purpose]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == t {
			return true
		}
	}
	return false
}
