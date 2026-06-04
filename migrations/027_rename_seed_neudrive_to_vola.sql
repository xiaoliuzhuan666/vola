-- Rename legacy seeded demo data from NeuDrive to Vola.

UPDATE roles
SET
    name = 'worker-vola',
    allowed_paths = ARRAY['/memory/projects/vola/**', '/skills/**']
WHERE user_id = 'a0000000-0000-0000-0000-000000000001'
  AND name = 'worker-neudrive'
  AND NOT EXISTS (
      SELECT 1
      FROM roles existing
      WHERE existing.user_id = roles.user_id
        AND existing.name = 'worker-vola'
  );

UPDATE roles
SET allowed_paths = ARRAY['/memory/projects/vola/**', '/skills/**']
WHERE user_id = 'a0000000-0000-0000-0000-000000000001'
  AND name = 'worker-vola';

UPDATE projects
SET
    name = 'vola',
    context_md = replace(replace(context_md, 'neuDrive', 'Vola'), 'NeuDrive', 'Vola'),
    updated_at = NOW()
WHERE id = 'b0000000-0000-0000-0000-000000000001'
  AND name = 'neudrive'
  AND NOT EXISTS (
      SELECT 1
      FROM projects existing
      WHERE existing.user_id = projects.user_id
        AND existing.name = 'vola'
  );

UPDATE projects
SET
    context_md = replace(replace(context_md, 'neuDrive', 'Vola'), 'NeuDrive', 'Vola'),
    updated_at = NOW()
WHERE id = 'b0000000-0000-0000-0000-000000000001';

UPDATE project_logs
SET summary = replace(replace(summary, 'neuDrive', 'Vola'), 'NeuDrive', 'Vola')
WHERE project_id = 'b0000000-0000-0000-0000-000000000001';

UPDATE activity_log
SET path = replace(path, '/memory/projects/neudrive/', '/memory/projects/vola/')
WHERE path LIKE '/memory/projects/neudrive/%';

UPDATE file_tree ft
SET path = '/projects/vola' || substring(ft.path from length('/projects/neudrive') + 1)
WHERE (ft.path = '/projects/neudrive' OR ft.path LIKE '/projects/neudrive/%')
  AND NOT EXISTS (
      SELECT 1
      FROM file_tree existing
      WHERE existing.user_id = ft.user_id
        AND existing.path = '/projects/vola' || substring(ft.path from length('/projects/neudrive') + 1)
  );

UPDATE file_tree
SET
    content = replace(replace(content, 'neuDrive', 'Vola'), 'NeuDrive', 'Vola'),
    updated_at = NOW()
WHERE content IS NOT NULL
  AND (content LIKE '%neuDrive%' OR content LIKE '%NeuDrive%');

UPDATE file_tree
SET checksum = encode(digest(
        coalesce(path, '') || '|' ||
        coalesce(content, '') || '|' ||
        coalesce(content_type, '') || '|' ||
        coalesce(metadata::text, '{}'),
        'sha256'
    ), 'hex')
WHERE path = '/projects/vola'
   OR path LIKE '/projects/vola/%'
   OR content LIKE '%Vola%';

UPDATE entry_versions ev
SET path = '/projects/vola' || substring(ev.path from length('/projects/neudrive') + 1)
WHERE ev.path = '/projects/neudrive'
   OR ev.path LIKE '/projects/neudrive/%';

UPDATE entry_versions
SET content = replace(replace(content, 'neuDrive', 'Vola'), 'NeuDrive', 'Vola')
WHERE content IS NOT NULL
  AND (content LIKE '%neuDrive%' OR content LIKE '%NeuDrive%');

UPDATE entry_versions
SET checksum = encode(digest(
        coalesce(path, '') || '|' ||
        coalesce(content, '') || '|' ||
        coalesce(content_type, '') || '|' ||
        coalesce(metadata::text, '{}'),
        'sha256'
    ), 'hex')
WHERE path = '/projects/vola'
   OR path LIKE '/projects/vola/%'
   OR content LIKE '%Vola%';
