# All-Chat Admin Dashboard

The admin dashboard provides a comprehensive interface for managing users, overlays, and chat sources across the All-Chat platform.

## Features

### 1. Dashboard Home (`/admin`)

- Quick access cards for Users, Overlays, and Sources
- Navigation to all admin sections
- Quick stats overview

### 2. Users Management (`/admin/users`)

- **User List**: View all registered users with their platform connections
- **Platform Badges**: Visual indicators for connected platforms (Twitch, YouTube, Kick, TikTok)
- **User Details Panel**:
  - User ID, username, email
  - Connected platform IDs
  - List of user's overlays
- **Selection**: Click on any user to view their details

### 3. Overlays Management (`/admin/overlays`)

- **Overlay List**: View all overlays across all users
- **Source Count**: See how many sources are connected to each overlay
- **Overlay Details Panel**:
  - Overlay name and ID
  - User ID
  - Creation date
- **Connected Sources Panel**:
  - Platform (Twitch, YouTube, Kick, TikTok)
  - Channel name and ID
  - Active/Inactive status
  - Creation date
- **Quick Actions**: Open overlay in new tab

### 4. Sources Management (`/admin/sources`)

- **Platform Statistics**: Cards showing count per platform
- **Advanced Filtering**:
  - Search by channel name or ID
  - Filter by platform (Twitch, YouTube, Kick, TikTok)
  - Filter by status (Active/Inactive)
- **Sources Table**: Comprehensive table with all sources
- **Quick Links**: Navigate to overlay details

## Navigation

The admin dashboard uses a consistent navigation bar with:

- **All-Chat Admin** brand/logo (links to dashboard home)
- **Users** - User management
- **Overlays** - Overlay and source management
- **Sources** - System-wide source overview
- **Back to App** - Return to main application

## Data Structure

### User

```typescript
{
  id: string;
  username: string;
  email: string;
  created_at: string;
  twitch_id?: string;
  youtube_id?: string;
  kick_id?: string;
  tiktok_id?: string;
}
```

### Overlay

```typescript
{
  id: string;
  name: string;
  user_id: string;
  created_at: string;
  updated_at: string;
  sources_count?: number;
}
```

### Source

```typescript
{
  id: string
  overlay_id: string
  overlay_name: string
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok'
  channel_id: string
  channel_name: string
  is_active: boolean
  created_at: string
  user_id: string
}
```

## API Integration

Currently using mock data for demonstration. To connect to real APIs:

### Users API

```typescript
// Fetch all users
GET / api / v1 / admin / users
Authorization: Bearer<token>

// Fetch user overlays
GET / api / v1 / overlays ? (user_id = <user_id>Authorization) : Bearer<token>
```

### Overlays API

```typescript
// Fetch all overlays (admin)
GET /api/v1/admin/overlays
Authorization: Bearer <token>

// Fetch overlay sources
GET /api/v1/overlays/:id/sources
Authorization: Bearer <token>
```

### Sources API

```typescript
// Fetch all sources (admin)
GET / api / v1 / admin / sources
Authorization: Bearer<token>
```

## Styling

The admin dashboard uses Tailwind CSS with:

- **Color Scheme**: Clean white backgrounds with gray accents
- **Platform Colors**:
  - Twitch: Purple (`bg-purple-100 text-purple-800`)
  - YouTube: Red (`bg-red-100 text-red-800`)
  - Kick: Green (`bg-green-100 text-green-800`)
  - TikTok: Pink (`bg-pink-100 text-pink-800`)
- **Status Colors**:
  - Active: Green
  - Inactive: Gray
- **Interactive Elements**: Hover effects on cards and rows

## Future Enhancements

1. **Backend Integration**:
   - Create admin-specific API endpoints
   - Implement user management actions (edit, delete, suspend)
   - Add overlay management actions (edit, delete)
   - Implement source management actions (activate, deactivate, delete)

2. **Real-time Updates**:
   - WebSocket integration for live source status
   - Real-time user activity monitoring
   - Live message count per source

3. **Analytics**:
   - User growth charts
   - Message volume per platform
   - Popular channels/overlays
   - System health metrics

4. **Authentication**:
   - Admin role verification
   - Permission-based access control
   - Audit logs for admin actions

5. **Advanced Features**:
   - Bulk operations
   - CSV export
   - Advanced search and filters
   - User impersonation (for support)

## Development

To run the admin dashboard locally:

```bash
cd frontend
npm install
npm run dev
```

Visit `http://localhost:3000/admin` to access the admin dashboard.

## Production Build

The admin pages are included in the main frontend build:

```bash
npm run build
```

The admin routes are pre-rendered as static content for optimal performance.
