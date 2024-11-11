package dynamic

type (
	NameService struct {
		Avatar string `json:"avatar"`
		Name   string `json:"name"`
	}
	WalletProperties struct {
		TurnkeySubOrganizationId string `json:"turnkeySubOrganizationId"`
		TurnkeyPrivateKeyId      string `json:"turnkeyPrivateKeyId"`
		TurnkeyHDWalletId        string `json:"turnkeyHDWalletId"`
		IsAuthenticatorAttached  bool   `json:"isAuthenticatorAttached"`
		TurnkeyUserId            string `json:"turnkeyUserId"`
		IsSessionKeyCompatible   bool   `json:"isSessionKeyCompatible"`
		Version                  string `json:"version"`
	}
	VerifiedCredential struct {
		Address                   string                 `json:"address"`
		Chain                     string                 `json:"chain"`
		RefId                     string                 `json:"refId"`
		SignerRefId               string                 `json:"signerRefId"`
		Email                     string                 `json:"email"`
		Id                        string                 `json:"id"`
		NameService               NameService            `json:"name_service"`
		PublicIdentifier          string                 `json:"public_identifier"`
		WalletName                string                 `json:"wallet_name"`
		WalletProvider            string                 `json:"wallet_provider"`
		WalletProperties          WalletProperties       `json:"wallet_properties"`
		Format                    string                 `json:"format"`
		OauthProvider             string                 `json:"oauth_provider"`
		OauthUsername             string                 `json:"oauth_username"`
		OauthDisplayName          string                 `json:"oauth_display_name"`
		OauthAccountId            string                 `json:"oauth_account_id"`
		PhoneNumber               string                 `json:"phoneNumber"`
		PhoneCountryCode          string                 `json:"phoneCountryCode"`
		IsoCountryCode            string                 `json:"isoCountryCode"`
		OauthAccountPhotos        []string               `json:"oauth_account_photos"`
		OauthEmails               []string               `json:"oauth_emails"`
		OauthMetadata             map[string]interface{} `json:"oauth_metadata"`
		PreviousUsers             []string               `json:"previous_users"`
		EmbeddedWalletId          string                 `json:"embedded_wallet_id"`
		WalletAdditionalAddresses []struct {
			Address   string `json:"address"`
			PublicKey string `json:"publicKey"`
			Type      string `json:"type"`
		} `json:"wallet_additional_addresses"`
		LastSelectedAt string `json:"lastSelectedAt"`
		SignInEnabled  bool   `json:"signInEnabled"`
	}
	MissingField struct {
		Name            string `json:"name"`
		Required        bool   `json:"required"`
		Enabled         bool   `json:"enabled"`
		Unique          bool   `json:"unique"`
		Verify          bool   `json:"verify"`
		Type            string `json:"type"`
		ValidationRules struct {
			Unique       bool   `json:"unique"`
			Regex        string `json:"regex"`
			ValidOptions []struct {
				Label string `json:"label"`
			} `json:"validOptions"`
			CheckboxText string `json:"checkboxText"`
		} `json:"validationRules"`
		ValidationType string `json:"validationType"`
		Label          string `json:"label"`
		Position       int    `json:"position"`
	}
	Session struct {
		Id        string `json:"id"`
		CreatedAt string `json:"createdAt"`
		IpAddress string `json:"ipAddress"`
		UserAgent string `json:"userAgent"`
		RevokedAt string `json:"revokedAt"`
	}
	Wallet struct {
		Id             string           `json:"id"`
		Name           string           `json:"name"`
		Chain          string           `json:"chain"`
		PublicKey      string           `json:"publicKey"`
		Provider       string           `json:"provider"`
		Properties     WalletProperties `json:"properties"`
		LastSelectedAt string           `json:"lastSelectedAt"`
	}
	ChainalysisCheck struct {
		Id              string `json:"id"`
		CreatedAt       string `json:"createdAt"`
		Result          string `json:"result"`
		WalletPublicKey string `json:"walletPublicKey"`
		Response        string `json:"response"`
	}
	OauthAccount struct {
		Id              string `json:"id"`
		Provider        string `json:"provider"`
		AccountUsername string `json:"accountUsername"`
	}
	MfaDevice struct {
		Type       string `json:"type"`
		Verified   bool   `json:"verified"`
		Id         string `json:"id"`
		CreatedAt  string `json:"createdAt"`
		VerifiedAt string `json:"verifiedAt"`
		Default    bool   `json:"default"`
		Alias      string `json:"alias"`
	}
	User struct {
		Id                           string                 `json:"id"`
		ProjectEnvironmentId         string                 `json:"projectEnvironmentId"`
		VerifiedCredentials          []VerifiedCredential   `json:"verifiedCredentials"`
		LastVerifiedCredentialId     string                 `json:"lastVerifiedCredentialId"`
		SessionId                    string                 `json:"sessionId"`
		Alias                        string                 `json:"alias"`
		Country                      string                 `json:"country"`
		Email                        string                 `json:"email"`
		FirstName                    string                 `json:"firstName"`
		JobTitle                     string                 `json:"jobTitle"`
		LastName                     string                 `json:"lastName"`
		PhoneNumber                  string                 `json:"phoneNumber"`
		PoliciesConsent              bool                   `json:"policiesConsent"`
		TShirtSize                   string                 `json:"tShirtSize"`
		Team                         string                 `json:"team"`
		Username                     string                 `json:"username"`
		FirstVisit                   string                 `json:"firstVisit"`
		LastVisit                    string                 `json:"lastVisit"`
		NewUser                      bool                   `json:"newUser"`
		Metadata                     map[string]interface{} `json:"metadata"`
		MfaBackupCodeAcknowledgement string                 `json:"mfaBackupCodeAcknowledgement"`
		BtcWallet                    string                 `json:"btcWallet"`
		KdaWallet                    string                 `json:"kdaWallet"`
		LtcWallet                    string                 `json:"ltcWallet"`
		CkbWallet                    string                 `json:"ckbWallet"`
		KasWallet                    string                 `json:"kasWallet"`
		DogeWallet                   string                 `json:"dogeWallet"`
		EmailNotification            bool                   `json:"emailNotification"`
		DiscordNotification          bool                   `json:"discordNotification"`
		NewsletterNotification       bool                   `json:"newsletterNotification"`
		Lists                        []string               `json:"lists"`
		Scope                        string                 `json:"scope"`
		MissingFields                []MissingField         `json:"missingFields"`
		WalletPublicKey              string                 `json:"walletPublicKey"`
		Wallet                       string                 `json:"wallet"`
		Chain                        string                 `json:"chain"`
		CreatedAt                    string                 `json:"createdAt"`
		UpdatedAt                    string                 `json:"updatedAt"`
		Sessions                     []Session              `json:"sessions"`
		Wallets                      []Wallet               `json:"wallets"`
		ChainalysisChecks            []ChainalysisCheck     `json:"chainalysisChecks"`
		OauthAccounts                []OauthAccount         `json:"oauthAccounts"`
		MfaDevices                   []MfaDevice            `json:"mfaDevices"`
	}
	UserWrapper struct {
		User User `json:"user"`
	}
)
