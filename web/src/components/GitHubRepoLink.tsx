import { useI18n } from '../i18n'

export const GITHUB_REPO_URL = 'https://github.com/agi-bar/neuDrive'

export default function GitHubRepoLink({ className = '' }: { className?: string }) {
  const { tx } = useI18n()
  return (
    <a
      className={`github-repo-link ${className}`.trim()}
      href={GITHUB_REPO_URL}
      target="_blank"
      rel="noreferrer"
      aria-label={tx('在 GitHub 查看 neuDrive', 'View neuDrive on GitHub')}
      title={tx('GitHub 仓库', 'GitHub repository')}
    >
      <svg className="github-repo-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
        <path d="M12 2C6.48 2 2 6.58 2 12.26c0 4.53 2.87 8.37 6.84 9.72.5.1.68-.22.68-.49 0-.24-.01-1.04-.01-1.89-2.78.62-3.37-1.21-3.37-1.21-.45-1.19-1.11-1.5-1.11-1.5-.91-.64.07-.63.07-.63 1 .07 1.53 1.06 1.53 1.06.89 1.57 2.34 1.12 2.91.85.09-.66.35-1.12.63-1.38-2.22-.26-4.56-1.14-4.56-5.07 0-1.12.39-2.03 1.03-2.75-.1-.26-.45-1.3.1-2.71 0 0 .84-.28 2.75 1.05A9.34 9.34 0 0 1 12 6.98c.85 0 1.71.12 2.51.34 1.91-1.33 2.75-1.05 2.75-1.05.55 1.41.2 2.45.1 2.71.64.72 1.03 1.63 1.03 2.75 0 3.94-2.34 4.81-4.57 5.07.36.32.68.94.68 1.9 0 1.38-.01 2.49-.01 2.83 0 .27.18.59.69.49A10.12 10.12 0 0 0 22 12.26C22 6.58 17.52 2 12 2Z" />
      </svg>
    </a>
  )
}
