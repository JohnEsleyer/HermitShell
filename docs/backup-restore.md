# Backup and Restore

Documentation for Hermit Agent OS backup and restore functionality.

## Overview

The backup and restore feature allows users to export all application data to a single `.zip` file and import it on a new system. This is useful for:
- Moving data to a new VPS
- Creating regular backups
- Disaster recovery

## What Data is Included

The backup includes:
- **Database** (`hermit.db`): All agents, settings, history, calendar events, skills, and credentials
- **Images** (`data/image/`): All uploaded images and assets
- **Skills** (`data/skills/`): Custom skills and context files
- **Agent Data** (`data/agents/`): Agent-specific skills and context
- **Logs** (`hermit.log`): Application logs

The following are NOT included:
- Database WAL and SHM files (SQLite temporary files)
- Docker containers (these are recreated from images on import)

## Security

### Export
- Export requires an active authenticated session
- No password is required for export (user controls their own data)

### Import
- **Password is required** for import to prevent unauthorized data restoration
- The password is verified against the user's stored credentials
- A warning is displayed before import to inform users about data overwriting

## API Endpoints

### Export Backup
```
GET /api/backup/export
```
Returns a `.zip` file containing all backup data.
- **Authentication**: Required (session cookie)
- **Response**: `application/zip` with `Content-Disposition: attachment`

### Import Backup
```
POST /api/backup/import
```
Accepts a `.zip` file and restores data.
- **Authentication**: Required (session cookie)
- **Content-Type**: `multipart/form-data`
- **Parameters**:
  - `backup`: The backup .zip file
  - `password`: User's password for verification
- **Response**: JSON with success/error status

## Usage

### From Dashboard
1. Go to **Settings** tab
2. Scroll to **Backup & Restore** section
3. For Export: Click "Download Backup" button
4. For Import: 
   - Select a backup .zip file
   - Enter your password
   - Click "Import Backup"
   - Restart the application if prompted

### Via API
```bash
# Export backup
curl -b cookies.txt -o hermit-backup.zip http://localhost:3000/api/backup/export

# Import backup
curl -b cookies.txt -X POST \
  -F "backup=@hermit-backup.zip" \
  -F "password=yourpassword" \
  http://localhost:3000/api/backup/import
```

## Limitations

1. **Docker containers are not restored**: Agent containers are not included in backups. They will be recreated from the Docker image when agents are started on the new system.
2. **API keys are preserved**: Settings including API keys are restored.
3. **Large files**: Very large databases or many images may take time to export/import.
4. **Version compatibility**: Backups are compatible within the same major version. Minor version differences should work but are not guaranteed.

## Best Practices

1. **Regular backups**: Create backups regularly, especially before major changes
2. **Test restore**: Periodically test restoring a backup to a test environment
3. **Secure storage**: Store backup files securely - they contain all your data including API keys
4. **Export before major changes**: Always export before updating to a new version or making significant changes
