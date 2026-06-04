"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.NeuDriveAuth = exports.VolaAuth = void 0;
/**
 * OAuth helper for third-party applications integrating with Vola.
 *
 * @example
 * ```ts
 * const auth = new VolaAuth({
 *   baseURL: 'https://vola.ai',
 *   clientId: 'your-client-id',
 *   clientSecret: 'your-client-secret',
 * })
 *
 * // Step 1: redirect user
 * const url = auth.getAuthorizationURL('https://yourapp.com/callback', ['read:profile'])
 *
 * // Step 2: exchange code after callback
 * const { access_token, user } = await auth.exchangeCode(code, 'https://yourapp.com/callback')
 * ```
 */
class VolaAuth {
    constructor(config) {
        if (!config.baseURL)
            throw new Error('VolaAuth: baseURL is required');
        if (!config.clientId)
            throw new Error('VolaAuth: clientId is required');
        if (!config.clientSecret)
            throw new Error('VolaAuth: clientSecret is required');
        this.baseURL = config.baseURL.replace(/\/+$/, '');
        this.clientId = config.clientId;
        this.clientSecret = config.clientSecret;
    }
    /**
     * Build the authorization URL that the user should be redirected to.
     *
     * @param redirectURI - Where Vola should redirect after the user authorises.
     * @param scopes      - Requested permission scopes (e.g. ["read:profile", "read:memory"]).
     * @returns A fully-qualified URL string.
     */
    getAuthorizationURL(redirectURI, scopes) {
        const params = new URLSearchParams({
            response_type: 'code',
            client_id: this.clientId,
            redirect_uri: redirectURI,
            scope: scopes.join(' '),
        });
        return `${this.baseURL}/oauth/authorize?${params.toString()}`;
    }
    /**
     * Exchange an authorisation code for an access token and user info.
     *
     * @param code        - The authorisation code received at the redirect URI.
     * @param redirectURI - Must match the redirect URI used in getAuthorizationURL.
     */
    async exchangeCode(code, redirectURI) {
        const res = await fetch(`${this.baseURL}/oauth/token`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                grant_type: 'authorization_code',
                code,
                redirect_uri: redirectURI,
                client_id: this.clientId,
                client_secret: this.clientSecret,
            }),
        });
        if (!res.ok) {
            let body;
            try {
                body = await res.json();
            }
            catch {
                body = await res.text();
            }
            const msg = typeof body === 'object' && body !== null && 'error' in body
                ? body.error
                : `HTTP ${res.status}`;
            throw new Error(`VolaAuth: token exchange failed: ${msg}`);
        }
        return (await res.json());
    }
    /**
     * Retrieve user information using an access token.
     *
     * @param accessToken - A valid access token (JWT or scoped token).
     */
    async getUserInfo(accessToken) {
        const res = await fetch(`${this.baseURL}/oauth/userinfo`, {
            method: 'GET',
            headers: {
                Authorization: `Bearer ${accessToken}`,
                'Content-Type': 'application/json',
            },
        });
        if (!res.ok) {
            let body;
            try {
                body = await res.json();
            }
            catch {
                body = await res.text();
            }
            const msg = typeof body === 'object' && body !== null && 'error' in body
                ? body.error
                : `HTTP ${res.status}`;
            throw new Error(`VolaAuth: getUserInfo failed: ${msg}`);
        }
        return (await res.json());
    }
}
exports.VolaAuth = VolaAuth;
exports.NeuDriveAuth = VolaAuth;
//# sourceMappingURL=auth.js.map