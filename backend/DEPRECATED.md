# ⚠️ DEPRECATED - DO NOT USE

## This backend has been replaced by backend2

**Status:** DEPRECATED as of 2024  
**Replacement:** `/backend2`

---

### Migration Notice

This legacy backend (`/backend`) has been completely replaced by the new backend2 implementation which features:

- ✅ Clean DDD/CQRS architecture
- ✅ Event sourcing with domain events
- ✅ API v2 endpoints (`/api/v2/*`)
- ✅ Improved error handling
- ✅ Better Lambda support
- ✅ Simplified dependency injection

### Do NOT use this backend for:
- ❌ New development
- ❌ Production deployments
- ❌ Local development
- ❌ Testing

### Instead, use backend2:
```bash
cd ../backend2
./run-local.sh  # For local development
```

### Frontend Integration
The frontend has been updated to use backend2's `/api/v2` endpoints.

### Category Support
Categories are not yet implemented in backend2 but will be added in a future update.

---

**This directory is kept for reference only and will be removed in a future cleanup.**