# User Module (Step 5)

## Routes (all under `/api/v1/users`, RequireAuth)

| Method | Path | Extra middleware | Description |
|--------|------|------------------|-------------|
| POST   | `/create-user` | — | Create user, regenerate session, set cookie, **201** with `data.user` |
| PUT    | `/update-user/:id` | — | Update allowed fields |
| GET    | `/view-user/:id` | — | Return user by ID |
| PATCH  | `/update-role` | **AuthorizePermission(assign_role)** | Update target user role; policy + session invalidation |
| DELETE | `/remove-user` | **AuthorizePermission(remove_user)** | Remove target from org; policy + session invalidation |

## Policies (replicated from Express)

- **`express/src/modules/user/policies/roleGovernance.js`** → `internal/user/policies/role_governance.go`  
  - `ValidateRoleAssignment`: only superadmin can assign admin/superadmin; same org; no self-role change (unless superadmin); last superadmin cannot be downgraded. Same error messages.
- **`express/src/modules/user/policies/membershipGovernance.js`** → `internal/user/policies/membership_governance.go`  
  - `ValidateOrgMembershipChange`: no self-removal; target in org; admin cannot remove admin/superadmin; last superadmin cannot be removed. Same error messages.
- **`express/src/modules/auth/policies/sessionInvalidation.js`** → `internal/user/policies/session_invalidation.go`  
  - `DestroyUserSessions`: SMEMBERS `user_sessions:<user_id>`, DEL each `session:<sid>`, DEL `user_sessions:<user_id>`. Called after update-role and remove-user.

Permission checks use **`constants.PermissionRoles`** (Express `permissionRoles.js`). Unconfigured permission → 500 "Permission configuration error"; role not allowed → 403 "User is Forbidden from performing this action".

## Run tests

```bash
go test ./internal/user/... -v
```

Includes: CreateUser requires auth (401), UpdateRole 403 for viewer, RemoveUser 403 for viewer, UpdateRole self-change 400, RemoveUser self-removal 400, and policy unit tests (role + membership).

## Postman verification

1. **Base**: Set `BASE_URL` (e.g. `http://localhost:3000`). All user routes need a session cookie: either call `POST /api/v1/auth/login` first and use the returned `Set-Cookie`, or call `POST /api/v1/users/create-user` and use that cookie for subsequent requests.

2. **Create user (201)**  
   - `POST {{BASE_URL}}/api/v1/users/create-user`  
   - Body (JSON): `{ "user_name": "alice", "email": "alice@example.com", "password": "Pass1!word", "fullname": "Alice Smith" }`  
   - Expect: **201**, `status: "success"`, `message: "User created successfully"`, `data.user` with `user_id`, `fullname`, `user_name`, `email`, `role`, `org_id`, no `password_hash`. Cookie `troo.sid` set.

3. **Update user**  
   - `PUT {{BASE_URL}}/api/v1/users/update-user/{{user_id}}`  
   - Body: `{ "fullname": "Alice Jones" }`  
   - Expect: 200, `data.user` updated.

4. **View user**  
   - `GET {{BASE_URL}}/api/v1/users/view-user/{{user_id}}`  
   - Expect: 200, `message: "User found"`, `data.user`.

5. **Update role – success (admin/superadmin)**  
   - Log in as a user with **admin** or **superadmin** (or create one and set role in DB to admin).  
   - `PATCH {{BASE_URL}}/api/v1/users/update-role`  
   - Body: `{ "user_id": "<target_user_uuid>", "role": "manager" }`  
   - Expect: 200, `data.user` with new role. Target’s sessions are invalidated.

6. **Update role – 403 (viewer/manager)**  
   - Log in as **viewer** or **manager**.  
   - Same `PATCH /api/v1/users/update-role`.  
   - Expect: **403**, `error.message: "User is Forbidden from performing this action"`.

7. **Update role – 400 (self-change)**  
   - Log in as **admin** (not superadmin).  
   - `PATCH /api/v1/users/update-role` with `user_id` = your own user_id, `role: "manager"`.  
   - Expect: **400**, `error.message: "Users cannot modify their own role"`.

8. **Remove user – success (admin/superadmin)**  
   - Log in as **admin** or **superadmin**.  
   - `DELETE {{BASE_URL}}/api/v1/users/remove-user`  
   - Body: `{ "user_id": "<target_user_uuid>" }` (target in same org, not yourself).  
   - Expect: 200, `message: "User removed from organization"`. Target’s sessions invalidated.

9. **Remove user – 403 (viewer/manager)**  
   - Log in as **viewer** or **manager**.  
   - Same `DELETE /api/v1/users/remove-user`.  
   - Expect: **403**, `error.message: "User is Forbidden from performing this action"`.

10. **Remove user – 400 (self-removal)**  
    - Log in as **admin**.  
    - `DELETE /api/v1/users/remove-user` with `user_id` = your own user_id.  
    - Expect: **400**, `error.message: "You cannot remove yourself from the organization"`.

## Policy logic confirmation

The Go implementation **fully replicates** the logic and error messages from:

- `./express/src/modules/user/policies/roleGovernance.js` (role assignment rules and messages).
- `./express/src/modules/user/policies/membershipGovernance.js` (membership change rules and messages).
- Permission checks match `./express/src/middleware/authorizePermission.js` and `./express/src/constants/permissionRoles.js` (403/500 and message parity).
- Session invalidation matches `./express/src/modules/auth/policies/sessionInvalidation.js` (keys and flow).
