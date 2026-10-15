# [ima.ge.cx](https://ima.ge.cx)

Browse the files, layers and configuration of public container images without
pulling them to your machine or running Docker.

- [frontend/](frontend/): Next.js web application, usage and development docs.
- [backend/](backend/): Go API, image indexing and AWS SAM infrastructure.

## Local development

For the frontend, use Node.js 20 or later:

```shell
cd frontend
npm ci
npm run dev
```

The frontend uses the hosted API by default. See its
[API configuration](frontend/README.md#api-configuration) to select another backend.

For backend development, initialize the gztool submodule from the repository
root, then run Go and SAM commands from `backend/`:

```shell
git submodule update --init backend/targzi/gztool
cd backend
sam build --parallel
```

The backend build requires Go and Docker/buildx. See the
[backend README](backend/README.md) for test requirements and deployment details.

## Deployment

The frontend and backend still deploy independently. The root
[amplify.yml](amplify.yml) builds the frontend in `frontend/`.
**Before pushing this layout to the deployed branch, set
`AMPLIFY_MONOREPO_APP_ROOT=frontend` in the existing Amplify app's environment
variables.** Pushing to `main` triggers the frontend's production deployment.

Backend infrastructure remains in `backend/template.yml`.
`backend/build.sh`, run from `backend/`, builds and deploys to AWS; it is not
a build-only validation command.

## Repository history

This repository combines the original `ima.ge.cx` frontend and
`ima.ge.cx-backend` main histories using a non-squashed subtree merge.
Original commits retain their hashes and original paths. New commits place the
applications under `frontend/` and `backend/`; application source is unchanged
by the move. The unused `newsite/ima.ge.cx` gitlink was removed, while gztool
remains a submodule at `backend/targzi/gztool`.
