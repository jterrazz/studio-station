package config

import (
	"context"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
)

const (
	dnsIdentifier = "com.jterrazz.dns.quad9"
	profilesPane  = "x-apple.systempreferences:com.apple.Profiles-Settings.extension"
)

func (s *Service) checkDNS(ctx context.Context) (state catalog.State, detail string) {
	if s.dnsInstalled(ctx) {
		return catalog.StateOK, "Quad9 DoH"
	}
	return catalog.StateAbsent, ""
}

func (s *Service) installDNS(ctx context.Context) (Change, error) {
	if s.dnsInstalled(ctx) {
		if _, err := s.must(ctx, "open", profilesPane); err != nil {
			return Change{}, err
		}
		return Change{Steps: []Step{okStep("profile", "already installed: Profiles settings opened")}}, nil
	}
	profile := s.o.DNSProfile
	if err := s.write(profile, []byte(dnsProfile()), 0o600); err != nil {
		return Change{}, err
	}
	if _, err := s.must(ctx, "open", profile); err != nil {
		return Change{}, err
	}
	return Change{Steps: []Step{
		okStep("profile", "written to "+profile),
		skipStep("next", "approve the profile in System Settings > Privacy & Security > Profiles"),
	}}, nil
}

func (s *Service) dnsInstalled(ctx context.Context) bool {
	out, ok := s.probe(ctx, "profiles", "-C", "-v")
	return ok && strings.Contains(out, dnsIdentifier)
}

func dnsProfile() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>PayloadContent</key>
	<array>
		<dict>
			<key>DNSSettings</key>
			<dict>
				<key>DNSProtocol</key>
				<string>HTTPS</string>
				<key>ServerAddresses</key>
				<array>
					<string>2620:fe::fe</string>
					<string>2620:fe::9</string>
					<string>9.9.9.9</string>
					<string>149.112.112.112</string>
				</array>
				<key>ServerURL</key>
				<string>https://dns.quad9.net/dns-query</string>
			</dict>
			<key>PayloadDescription</key>
			<string>Configures device to use Quad9 Encrypted DNS over HTTPS</string>
			<key>PayloadDisplayName</key>
			<string>Quad9 DNS over HTTPS</string>
			<key>PayloadIdentifier</key>
			<string>` + dnsIdentifier + `.doh</string>
			<key>PayloadType</key>
			<string>com.apple.dnsSettings.managed</string>
			<key>PayloadUUID</key>
			<string>B9C4F3A2-5E6D-7F8A-9B0C-1D2E3F4A5B6C</string>
			<key>PayloadVersion</key>
			<integer>1</integer>
			<key>ProhibitDisablement</key>
			<false/>
		</dict>
	</array>
	<key>PayloadDescription</key>
	<string>Configures encrypted DNS over HTTPS using Quad9 (9.9.9.9)</string>
	<key>PayloadDisplayName</key>
	<string>Quad9 Encrypted DNS</string>
	<key>PayloadIdentifier</key>
	<string>` + dnsIdentifier + `</string>
	<key>PayloadRemovalDisallowed</key>
	<false/>
	<key>PayloadScope</key>
	<string>System</string>
	<key>PayloadType</key>
	<string>Configuration</string>
	<key>PayloadUUID</key>
	<string>A8B3E2F1-4D5C-6E7F-8A9B-0C1D2E3F4A5B</string>
	<key>PayloadVersion</key>
	<integer>1</integer>
</dict>
</plist>`
}
