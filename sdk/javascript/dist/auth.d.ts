import type { User, AuthTokenResponse } from './types';
export interface NeuDriveAuthConfig {
    /** Base URL of the Vola instance */
    baseURL: string;
    /** OAuth client ID */
    clientId: string;
    /** OAuth client secret */
    clientSecret: string;
}
export type VolaAuthConfig = NeuDriveAuthConfig;
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
export declare class VolaAuth {
    private readonly baseURL;
    private readonly clientId;
    private readonly clientSecret;
    constructor(config: VolaAuthConfig);
    /**
     * Build the authorization URL that the user should be redirected to.
     *
     * @param redirectURI - Where Vola should redirect after the user authorises.
     * @param scopes      - Requested permission scopes (e.g. ["read:profile", "read:memory"]).
     * @returns A fully-qualified URL string.
     */
    getAuthorizationURL(redirectURI: string, scopes: string[]): string;
    /**
     * Exchange an authorisation code for an access token and user info.
     *
     * @param code        - The authorisation code received at the redirect URI.
     * @param redirectURI - Must match the redirect URI used in getAuthorizationURL.
     */
    exchangeCode(code: string, redirectURI: string): Promise<AuthTokenResponse>;
    /**
     * Retrieve user information using an access token.
     *
     * @param accessToken - A valid access token (JWT or scoped token).
     */
    getUserInfo(accessToken: string): Promise<User>;
}
export { VolaAuth as NeuDriveAuth };
//# sourceMappingURL=auth.d.ts.map