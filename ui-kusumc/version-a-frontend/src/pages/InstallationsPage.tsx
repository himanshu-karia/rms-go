import { ChangeEvent, FormEvent, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  assignBeneficiaryToInstallation,
  ApiError,
  createBeneficiary,
  fetchBeneficiaries,
  fetchInstallationAssignments,
  fetchInstallations,
  removeBeneficiaryAssignment,
  updateBeneficiary,
  type Beneficiary,
  type BeneficiaryContact,
  type BeneficiaryContactInput,
  type BeneficiaryLocationInput,
  type CreateBeneficiaryPayload,
  type GeoLocationInput,
  type Installation,
  type InstallationAssignment,
  type UpdateBeneficiaryPayload,
} from '../api/installations';
import { StatusBadge } from '../components/StatusBadge';
import { useAuth } from '../auth';

function formatDate(value: string | null | undefined): string {
  if (!value) {
    return '—';
  }

  const timestamp = Date.parse(value);
  if (Number.isNaN(timestamp)) {
    return '—';
  }

  return new Date(timestamp).toLocaleString();
}

function sortAssignments(assignments: InstallationAssignment[]): InstallationAssignment[] {
  return [...assignments].sort((a, b) => {
    if (a.assignmentStatus !== b.assignmentStatus) {
      return a.assignmentStatus === 'active' ? -1 : 1;
    }
    if (a.role !== b.role) {
      return a.role === 'owner' ? -1 : 1;
    }
    return a.beneficiary.name.localeCompare(b.beneficiary.name);
  });
}

type NewBeneficiaryFormState = {
  name: string;
  email: string;
  phoneNumber: string;
  emailProxyBeneficiaryUuid: string;
  phoneProxyBeneficiaryUuid: string;
  address: string;
  state: string;
  district: string;
  tehsil: string;
  village: string;
  latitude: string;
  longitude: string;
  accuracyMeters: string;
  locationSource: string;
  locationCapturedAt: string;
};

function makeEmptyBeneficiaryForm(): NewBeneficiaryFormState {
  return {
    name: '',
    email: '',
    phoneNumber: '',
    emailProxyBeneficiaryUuid: '',
    phoneProxyBeneficiaryUuid: '',
    address: '',
    state: '',
    district: '',
    tehsil: '',
    village: '',
    latitude: '',
    longitude: '',
    accuracyMeters: '',
    locationSource: '',
    locationCapturedAt: '',
  };
}

const MAX_PHONE_CONTACTS = 3;

function toDateTimeLocalValue(value: string | null | undefined): string {
  if (!value) {
    return '';
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '';
  }
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  return `${year}-${month}-${day}T${hours}:${minutes}`;
}

type NormalizedBeneficiaryFormResult =
  | { error: string }
  | {
      error: null;
      name: string;
      email: string | null;
      phoneNumber: string | null;
      address: string | null;
      contacts: BeneficiaryContactInput[];
      location: BeneficiaryLocationInput | null;
      raw: {
        email: string;
        phoneNumber: string;
      };
    };

type NormalizedBeneficiaryFormSuccess = Exclude<NormalizedBeneficiaryFormResult, { error: string }>;

type ContactConflictDetails = {
  type: 'phone' | 'email';
  value: string;
  beneficiaryUuid: string;
};

type ContactConflict = ContactConflictDetails & {
  message: string;
};

function isContactConflictDetails(details: unknown): details is ContactConflictDetails {
  if (!details || typeof details !== 'object') {
    return false;
  }
  const candidate = details as Record<string, unknown>;
  return (
    (candidate.type === 'phone' || candidate.type === 'email') &&
    typeof candidate.value === 'string' &&
    typeof candidate.beneficiaryUuid === 'string'
  );
}

function isContactConflictError(
  error: unknown,
): error is ApiError & { details: ContactConflictDetails } {
  if (!(error instanceof Error)) {
    return false;
  }
  const apiError = error as ApiError;
  return apiError.status === 409 && isContactConflictDetails(apiError.details);
}

function formatBeneficiaryUuid(uuid: string): string {
  if (!uuid) {
    return '';
  }
  return uuid.length > 8 ? `${uuid.slice(0, 8)}…` : uuid;
}

function normalizeBeneficiaryForm(form: NewBeneficiaryFormState): NormalizedBeneficiaryFormResult {
  const name = form.name.trim();
  if (!name) {
    return { error: 'Beneficiary name is required.' };
  }

  const email = form.email.trim();
  const phoneNumber = form.phoneNumber.trim();
  const emailProxy = form.emailProxyBeneficiaryUuid.trim();
  const phoneProxy = form.phoneProxyBeneficiaryUuid.trim();
  const address = form.address.trim();
  const state = form.state.trim();
  const district = form.district.trim();
  const tehsil = form.tehsil.trim();
  const village = form.village.trim();
  const latitude = form.latitude.trim();
  const longitude = form.longitude.trim();
  const accuracyMeters = form.accuracyMeters.trim();
  const locationSource = form.locationSource.trim();
  const capturedAtInput = form.locationCapturedAt.trim();

  const contacts: BeneficiaryContactInput[] = [];
  if (phoneNumber) {
    const contact: BeneficiaryContactInput = {
      type: 'phone',
      value: phoneNumber,
      isPrimary: true,
    };
    if (phoneProxy) {
      contact.proxyBeneficiaryUuid = phoneProxy;
    }
    contacts.push(contact);
  }
  if (email) {
    const contact: BeneficiaryContactInput = {
      type: 'email',
      value: email,
      isPrimary: true,
    };
    if (emailProxy) {
      contact.proxyBeneficiaryUuid = emailProxy;
    }
    contacts.push(contact);
  }

  const hasGeoInputs = Boolean(
    latitude || longitude || accuracyMeters || locationSource || capturedAtInput,
  );

  let geoLocation: GeoLocationInput | null = null;

  if (hasGeoInputs) {
    if (!latitude || !longitude) {
      return {
        error: 'Latitude and longitude are required when providing geo-location details.',
      };
    }

    const parsedLatitude = Number.parseFloat(latitude);
    const parsedLongitude = Number.parseFloat(longitude);

    if (!Number.isFinite(parsedLatitude) || !Number.isFinite(parsedLongitude)) {
      return { error: 'Latitude and longitude must be valid numbers.' };
    }

    let accuracy: number | undefined;
    if (accuracyMeters) {
      const parsedAccuracy = Number.parseFloat(accuracyMeters);
      if (!Number.isFinite(parsedAccuracy) || parsedAccuracy < 0) {
        return { error: 'Accuracy must be a non-negative number.' };
      }
      accuracy = parsedAccuracy;
    }

    let capturedAtIso: string | undefined;
    if (capturedAtInput) {
      const parsedDate = new Date(capturedAtInput);
      if (Number.isNaN(parsedDate.getTime())) {
        return { error: 'Geo capture time must be a valid date and time.' };
      }
      capturedAtIso = parsedDate.toISOString();
    }

    geoLocation = {
      latitude: parsedLatitude,
      longitude: parsedLongitude,
      source: locationSource ? locationSource : null,
    };

    if (accuracy !== undefined) {
      geoLocation.accuracyMeters = accuracy;
    }
    if (capturedAtIso) {
      geoLocation.capturedAt = capturedAtIso;
    }
  }

  const hasLocation = Boolean(state || district || tehsil || village || geoLocation);
  const location: BeneficiaryLocationInput | null = hasLocation
    ? {
        state: state || null,
        district: district || null,
        tehsil: tehsil || null,
        village: village || null,
        ...(geoLocation ? { geoLocation } : {}),
      }
    : null;

  return {
    error: null,
    name,
    email: email ? email : null,
    phoneNumber: phoneNumber ? phoneNumber : null,
    address: address ? address : null,
    contacts,
    location,
    raw: {
      email,
      phoneNumber,
    },
  };
}

type DisplayContact = {
  key: string;
  label: string;
  isPrimary: boolean;
};

function buildContactLabel(contact: BeneficiaryContact): DisplayContact {
  let label = `${contact.type === 'phone' ? 'Phone' : 'Email'}: ${contact.value}`;
  if (contact.isPrimary) {
    label += ' (primary)';
  }
  if (contact.proxyBeneficiaryUuid) {
    label += ` · proxy ${contact.proxyBeneficiaryUuid.slice(0, 8)}…`;
  }
  return {
    key: `${contact.type}:${contact.value}:${contact.proxyBeneficiaryUuid ?? 'self'}`,
    label,
    isPrimary: Boolean(contact.isPrimary),
  };
}

function getDisplayContacts(beneficiary: Beneficiary): DisplayContact[] {
  if (beneficiary.contacts?.length) {
    return beneficiary.contacts.map((contact) => buildContactLabel(contact));
  }

  const fallbacks: DisplayContact[] = [];

  if (beneficiary.phoneNumber) {
    fallbacks.push({
      key: `phone:${beneficiary.phoneNumber}`,
      label: `Phone: ${beneficiary.phoneNumber}`,
      isPrimary: true,
    });
  }

  if (beneficiary.email) {
    fallbacks.push({
      key: `email:${beneficiary.email}`,
      label: `Email: ${beneficiary.email}`,
      isPrimary: true,
    });
  }

  return fallbacks;
}

function resolvePrimaryContactValue(beneficiary: Beneficiary, type: 'phone' | 'email'): string {
  const match = beneficiary.contacts?.find((contact) => contact.type === type && contact.isPrimary);
  if (match) {
    return match.value;
  }
  if (type === 'phone') {
    return beneficiary.phoneNumber ?? '';
  }
  return beneficiary.email ?? '';
}

function resolvePrimaryContactProxy(beneficiary: Beneficiary, type: 'phone' | 'email'): string {
  const match = beneficiary.contacts?.find((contact) => contact.type === type && contact.isPrimary);
  if (match?.proxyBeneficiaryUuid) {
    return match.proxyBeneficiaryUuid;
  }
  return '';
}

function populateFormFromBeneficiary(beneficiary: Beneficiary): NewBeneficiaryFormState {
  const location = beneficiary.location;
  const geo = location?.geoLocation;
  return {
    name: beneficiary.name,
    email: resolvePrimaryContactValue(beneficiary, 'email'),
    phoneNumber: resolvePrimaryContactValue(beneficiary, 'phone'),
    emailProxyBeneficiaryUuid: resolvePrimaryContactProxy(beneficiary, 'email'),
    phoneProxyBeneficiaryUuid: resolvePrimaryContactProxy(beneficiary, 'phone'),
    address: beneficiary.address ?? '',
    state: location?.state ?? '',
    district: location?.district ?? '',
    tehsil: location?.tehsil ?? '',
    village: location?.village ?? '',
    latitude: geo?.latitude !== null && geo?.latitude !== undefined ? String(geo.latitude) : '',
    longitude: geo?.longitude !== null && geo?.longitude !== undefined ? String(geo.longitude) : '',
    accuracyMeters:
      geo?.accuracyMeters !== null && geo?.accuracyMeters !== undefined
        ? String(geo.accuracyMeters)
        : '',
    locationSource: geo?.source ?? '',
    locationCapturedAt: toDateTimeLocalValue(geo?.capturedAt ?? null),
  };
}

type ContactLike = {
  type: 'phone' | 'email';
  value: string;
  proxyBeneficiaryUuid?: string | null;
};

function getContactKey(contact: ContactLike): string {
  return `${contact.type}:${contact.value}:${contact.proxyBeneficiaryUuid ?? 'self'}`;
}

function deduplicateContacts(contacts: BeneficiaryContactInput[]): BeneficiaryContactInput[] {
  const seen = new Set<string>();
  const result: BeneficiaryContactInput[] = [];
  for (const contact of contacts) {
    const key = getContactKey(contact);
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    result.push({
      ...contact,
      isPrimary: contact.isPrimary ?? false,
      proxyBeneficiaryUuid: contact.proxyBeneficiaryUuid ?? null,
    });
  }
  return result;
}

function ensurePrimaryPerType(contacts: BeneficiaryContactInput[]): BeneficiaryContactInput[] {
  const result = [...contacts];
  (['phone', 'email'] as const).forEach((type) => {
    let primaryFound = false;
    for (let index = 0; index < result.length; index += 1) {
      const contact = result[index];
      if (contact.type !== type) {
        continue;
      }
      if (contact.isPrimary && !primaryFound) {
        primaryFound = true;
        continue;
      }
      if (contact.isPrimary && primaryFound) {
        result[index] = { ...contact, isPrimary: false };
      }
    }
    if (!primaryFound) {
      const firstIndex = result.findIndex((contact) => contact.type === type);
      if (firstIndex >= 0) {
        result[firstIndex] = { ...result[firstIndex], isPrimary: true };
      }
    }
  });
  return result;
}

function enforcePhoneLimit(contacts: BeneficiaryContactInput[]): BeneficiaryContactInput[] {
  const phoneContacts = contacts.filter((contact) => contact.type === 'phone');
  if (phoneContacts.length <= MAX_PHONE_CONTACTS) {
    return contacts;
  }

  const primary = phoneContacts.filter((contact) => contact.isPrimary);
  const nonPrimary = phoneContacts.filter((contact) => !contact.isPrimary);
  const prioritized = [...primary, ...nonPrimary].slice(0, MAX_PHONE_CONTACTS);
  const allowedKeys = new Set(prioritized.map((contact) => getContactKey(contact)));

  return contacts.filter(
    (contact) => contact.type !== 'phone' || allowedKeys.has(getContactKey(contact)),
  );
}

function mergeContactsForUpdate(
  beneficiary: Beneficiary,
  primaryContacts: BeneficiaryContactInput[],
): BeneficiaryContactInput[] {
  const carryOver: BeneficiaryContactInput[] = (beneficiary.contacts ?? [])
    .filter((contact) => {
      if ((contact.type === 'phone' || contact.type === 'email') && contact.isPrimary) {
        return false;
      }
      return true;
    })
    .map((contact) => ({
      type: contact.type,
      value: contact.value,
      isPrimary: contact.isPrimary,
      proxyBeneficiaryUuid: contact.proxyBeneficiaryUuid ?? null,
    }));

  let merged: BeneficiaryContactInput[] = [
    ...carryOver,
    ...primaryContacts.map((contact) => ({
      type: contact.type,
      value: contact.value,
      isPrimary: contact.isPrimary ?? false,
      proxyBeneficiaryUuid: contact.proxyBeneficiaryUuid ?? null,
    })),
  ];

  merged = deduplicateContacts(merged);
  merged = ensurePrimaryPerType(merged);
  merged = enforcePhoneLimit(merged);
  return merged;
}

type BeneficiaryFormProps = {
  title: string;
  formState: NewBeneficiaryFormState;
  setFormState: React.Dispatch<React.SetStateAction<NewBeneficiaryFormState>>;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  submitting: boolean;
  submitLabel: string;
  submittingLabel: string;
  clientError: string | null;
  serverError: Error | null;
  helperText?: string;
  onCancel?: () => void;
  cancelLabel?: string;
  conflict?: ContactConflict | null;
  onLinkConflictAsProxy?: (conflict: ContactConflict) => void;
  onConflictDismiss?: () => void;
  onContactValueChange?: (type: 'phone' | 'email', value: string) => void;
  onClearProxy?: (type: 'phone' | 'email') => void;
};

function BeneficiaryForm({
  title,
  formState,
  setFormState,
  onSubmit,
  submitting,
  submitLabel,
  submittingLabel,
  clientError,
  serverError,
  helperText,
  onCancel,
  cancelLabel,
  conflict,
  onLinkConflictAsProxy,
  onConflictDismiss,
  onContactValueChange,
  onClearProxy,
}: BeneficiaryFormProps) {
  const updateField = (
    field: keyof NewBeneficiaryFormState,
  ): ((event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => void) => {
    return (event: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
      const value = event.target.value;
      setFormState((prev) => ({ ...prev, [field]: value }));
      if (field === 'phoneNumber' || field === 'email') {
        const type = field === 'phoneNumber' ? 'phone' : 'email';
        onContactValueChange?.(type, value);
      }
    };
  };

  const phoneProxyUuid = formState.phoneProxyBeneficiaryUuid;
  const emailProxyUuid = formState.emailProxyBeneficiaryUuid;
  const conflictTypeLabel = conflict?.type === 'email' ? 'email address' : 'phone number';

  return (
    <form
      className="flex flex-col gap-2 rounded border border-emerald-200 bg-white p-3"
      onSubmit={onSubmit}
    >
      <h6 className="text-sm font-semibold text-emerald-700">{title}</h6>
      <input
        type="text"
        value={formState.name}
        onChange={updateField('name')}
        placeholder="Beneficiary name"
        className="rounded border border-emerald-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-200"
        required
      />
      <input
        type="email"
        value={formState.email}
        onChange={updateField('email')}
        placeholder="Email (optional)"
        className="rounded border border-slate-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-200"
      />
      {emailProxyUuid && (
        <div className="flex items-center justify-between rounded border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs text-emerald-700">
          <span>Linked as proxy to {formatBeneficiaryUuid(emailProxyUuid)}</span>
          <button
            type="button"
            className="font-semibold text-emerald-700 underline hover:no-underline"
            onClick={() => onClearProxy?.('email')}
          >
            Remove proxy
          </button>
        </div>
      )}
      <input
        type="text"
        value={formState.phoneNumber}
        onChange={updateField('phoneNumber')}
        placeholder="Phone number (optional)"
        className="rounded border border-slate-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-200"
      />
      {phoneProxyUuid && (
        <div className="flex items-center justify-between rounded border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs text-emerald-700">
          <span>Linked as proxy to {formatBeneficiaryUuid(phoneProxyUuid)}</span>
          <button
            type="button"
            className="font-semibold text-emerald-700 underline hover:no-underline"
            onClick={() => onClearProxy?.('phone')}
          >
            Remove proxy
          </button>
        </div>
      )}
      <textarea
        value={formState.address}
        onChange={updateField('address')}
        placeholder="Address (optional)"
        className="min-h-[80px] rounded border border-slate-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-200"
      />

      {helperText && <p className="mt-1 text-xs text-slate-500">{helperText}</p>}

      <div className="grid gap-2 sm:grid-cols-2">
        <input
          type="text"
          value={formState.state}
          onChange={updateField('state')}
          placeholder="State (optional)"
          className="rounded border border-slate-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-200"
        />
        <input
          type="text"
          value={formState.district}
          onChange={updateField('district')}
          placeholder="District (optional)"
          className="rounded border border-slate-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-200"
        />
        <input
          type="text"
          value={formState.tehsil}
          onChange={updateField('tehsil')}
          placeholder="Tehsil (optional)"
          className="rounded border border-slate-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-200"
        />
        <input
          type="text"
          value={formState.village}
          onChange={updateField('village')}
          placeholder="Village (optional)"
          className="rounded border border-slate-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-200"
        />
      </div>

      <div className="grid gap-2 sm:grid-cols-2">
        <input
          type="number"
          step="any"
          value={formState.latitude}
          onChange={updateField('latitude')}
          placeholder="Latitude (optional)"
          className="rounded border border-slate-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-200"
        />
        <input
          type="number"
          step="any"
          value={formState.longitude}
          onChange={updateField('longitude')}
          placeholder="Longitude (optional)"
          className="rounded border border-slate-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-200"
        />
        <input
          type="number"
          step="any"
          value={formState.accuracyMeters}
          onChange={updateField('accuracyMeters')}
          placeholder="Accuracy (meters, optional)"
          className="rounded border border-slate-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-200"
        />
        <input
          type="text"
          value={formState.locationSource}
          onChange={updateField('locationSource')}
          placeholder="Location source (optional)"
          className="rounded border border-slate-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-200"
        />
      </div>

      <input
        type="datetime-local"
        aria-label="Geo capture time (optional)"
        value={formState.locationCapturedAt}
        onChange={updateField('locationCapturedAt')}
        className="rounded border border-slate-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-200"
      />

      {conflict && (
        <div className="rounded border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
          <p className="font-semibold">{conflict.message}</p>
          <p className="mt-1 text-xs">
            Beneficiary {formatBeneficiaryUuid(conflict.beneficiaryUuid)} already uses this{' '}
            {conflictTypeLabel}.
          </p>
          <div className="mt-2 flex flex-wrap gap-2">
            {onLinkConflictAsProxy && (
              <button
                type="button"
                className="rounded bg-emerald-600 px-3 py-2 text-xs font-semibold text-white hover:bg-emerald-500"
                onClick={() => onLinkConflictAsProxy(conflict)}
              >
                Link as proxy
              </button>
            )}
            {onConflictDismiss && (
              <button
                type="button"
                className="rounded border border-amber-300 px-3 py-2 text-xs font-semibold text-amber-700 hover:bg-amber-100"
                onClick={onConflictDismiss}
              >
                Use different {conflictTypeLabel}
              </button>
            )}
          </div>
        </div>
      )}

      {clientError && (
        <div className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          {clientError}
        </div>
      )}

      {serverError && !conflict && (
        <div className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
          {serverError.message}
        </div>
      )}

      <div className="flex items-center gap-2">
        <button
          type="submit"
          className="rounded bg-emerald-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-60"
          disabled={submitting}
        >
          {submitting ? submittingLabel : submitLabel}
        </button>
        {onCancel && (
          <button
            className="rounded border border-slate-300 px-3 py-2 text-sm font-medium text-slate-600 transition-colors hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
            onClick={onCancel}
            disabled={submitting}
          >
            {cancelLabel ?? 'Cancel'}
          </button>
        )}
      </div>
    </form>
  );
}

export function InstallationsPage() {
  const queryClient = useQueryClient();
  const { hasCapability } = useAuth();
  const [searchInput, setSearchInput] = useState('');
  const [search, setSearch] = useState('');
  const [explicitInstallationUuid, setExplicitInstallationUuid] = useState<string | null>(null);
  const [beneficiarySearchInput, setBeneficiarySearchInput] = useState('');
  const [beneficiarySearch, setBeneficiarySearch] = useState('');
  const [explicitBeneficiaryUuid, setExplicitBeneficiaryUuid] = useState<string | null>(null);
  const [assignmentRole, setAssignmentRole] = useState<'owner' | 'secondary'>('secondary');
  const [showCreateBeneficiaryForm, setShowCreateBeneficiaryForm] = useState(false);
  const [newBeneficiary, setNewBeneficiary] =
    useState<NewBeneficiaryFormState>(makeEmptyBeneficiaryForm);
  const [createFormError, setCreateFormError] = useState<string | null>(null);
  const [createConflict, setCreateConflict] = useState<ContactConflict | null>(null);
  const [editingBeneficiary, setEditingBeneficiary] = useState<Beneficiary | null>(null);
  const [editBeneficiary, setEditBeneficiary] =
    useState<NewBeneficiaryFormState>(makeEmptyBeneficiaryForm);
  const [editFormError, setEditFormError] = useState<string | null>(null);
  const [editConflict, setEditConflict] = useState<ContactConflict | null>(null);

  const installationsQueryKey = useMemo(() => ['installations', { search }], [search]);
  const installationsQuery = useQuery<Installation[], Error>({
    queryKey: installationsQueryKey,
    queryFn: () => fetchInstallations(search ? { search } : undefined),
    placeholderData: (previousData) => previousData ?? [],
  });

  const installations = useMemo(
    () => installationsQuery.data ?? [],
    [installationsQuery.data],
  ) as Installation[];

  const selectedInstallationUuid = useMemo(() => {
    if (!installations.length) {
      return null;
    }

    if (
      explicitInstallationUuid &&
      installations.some((installation) => installation.uuid === explicitInstallationUuid)
    ) {
      return explicitInstallationUuid;
    }

    return installations[0].uuid;
  }, [explicitInstallationUuid, installations]);

  const selectedInstallation = useMemo(() => {
    if (!selectedInstallationUuid) {
      return null;
    }
    return installations.find((item) => item.uuid === selectedInstallationUuid) ?? null;
  }, [installations, selectedInstallationUuid]);

  const assignmentsQueryKey = useMemo(
    () => ['installationAssignments', selectedInstallation?.uuid],
    [selectedInstallation?.uuid],
  );

  const assignmentsQuery = useQuery<InstallationAssignment[], Error>({
    queryKey: assignmentsQueryKey,
    queryFn: () => fetchInstallationAssignments(selectedInstallationUuid!),
    enabled: Boolean(selectedInstallationUuid),
  });

  const assignments = useMemo(
    () => sortAssignments(assignmentsQuery.data ?? []),
    [assignmentsQuery.data],
  );

  const beneficiariesQueryKey = useMemo(
    () => ['beneficiaries', { search: beneficiarySearch }],
    [beneficiarySearch],
  );

  const beneficiariesQuery = useQuery<Beneficiary[], Error>({
    queryKey: beneficiariesQueryKey,
    queryFn: () => fetchBeneficiaries({ search: beneficiarySearch || undefined, limit: 100 }),
  });

  const beneficiaries = useMemo(
    () => beneficiariesQuery.data ?? [],
    [beneficiariesQuery.data],
  ) as Beneficiary[];

  const availableBeneficiaries = useMemo(() => {
    if (!assignments.length) {
      return beneficiaries;
    }

    const assignedActive = new Set(
      assignments
        .filter((item) => item.assignmentStatus === 'active')
        .map((item) => item.beneficiaryUuid),
    );

    return beneficiaries.filter((beneficiary) => !assignedActive.has(beneficiary.uuid));
  }, [assignments, beneficiaries]);

  const selectedBeneficiaryUuid = useMemo(() => {
    if (!availableBeneficiaries.length) {
      return null;
    }

    if (
      explicitBeneficiaryUuid &&
      availableBeneficiaries.some((beneficiary) => beneficiary.uuid === explicitBeneficiaryUuid)
    ) {
      return explicitBeneficiaryUuid;
    }

    return availableBeneficiaries[0].uuid;
  }, [availableBeneficiaries, explicitBeneficiaryUuid]);

  type AssignBeneficiaryVariables = {
    installationUuid: string;
    beneficiaryUuid: string;
    role: 'owner' | 'secondary';
  };

  const assignMutation = useMutation({
    mutationFn: (variables: AssignBeneficiaryVariables) =>
      assignBeneficiaryToInstallation(variables.installationUuid, {
        beneficiaryUuid: variables.beneficiaryUuid,
        role: variables.role,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: assignmentsQueryKey });
      queryClient.invalidateQueries({ queryKey: installationsQueryKey });
      queryClient.invalidateQueries({ queryKey: beneficiariesQueryKey });
      setAssignmentRole('secondary');
    },
  });

  type RemoveAssignmentVariables = {
    installationUuid: string;
    beneficiaryUuid: string;
  };

  const removeAssignmentMutation = useMutation({
    mutationFn: (variables: RemoveAssignmentVariables) =>
      removeBeneficiaryAssignment(variables.installationUuid, variables.beneficiaryUuid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: assignmentsQueryKey });
      queryClient.invalidateQueries({ queryKey: installationsQueryKey });
      queryClient.invalidateQueries({ queryKey: beneficiariesQueryKey });
    },
  });

  type UpdateBeneficiaryVariables = {
    beneficiaryUuid: string;
    payload: UpdateBeneficiaryPayload;
  };

  const createBeneficiaryMutation = useMutation<Beneficiary, ApiError, CreateBeneficiaryPayload>({
    mutationFn: (payload) => createBeneficiary(payload),
    onSuccess: (beneficiary: Beneficiary) => {
      setShowCreateBeneficiaryForm(false);
      setCreateFormError(null);
      setCreateConflict(null);
      setNewBeneficiary(makeEmptyBeneficiaryForm());
      setExplicitBeneficiaryUuid(beneficiary.uuid);
      queryClient.invalidateQueries({ queryKey: beneficiariesQueryKey });
    },
    onError: (error) => {
      if (isContactConflictError(error)) {
        setCreateConflict({ ...error.details, message: error.message });
        return;
      }
      setCreateConflict(null);
    },
  });

  const updateBeneficiaryMutation = useMutation<Beneficiary, ApiError, UpdateBeneficiaryVariables>({
    mutationFn: (variables) => updateBeneficiary(variables.beneficiaryUuid, variables.payload),
    onSuccess: (beneficiary: Beneficiary) => {
      setEditFormError(null);
      setEditConflict(null);
      setEditingBeneficiary(null);
      setEditBeneficiary(makeEmptyBeneficiaryForm());
      setExplicitBeneficiaryUuid(beneficiary.uuid);
      queryClient.invalidateQueries({ queryKey: beneficiariesQueryKey });
      queryClient.invalidateQueries({ queryKey: assignmentsQueryKey });
      queryClient.invalidateQueries({ queryKey: installationsQueryKey });
    },
    onError: (error) => {
      if (isContactConflictError(error)) {
        setEditConflict({ ...error.details, message: error.message });
        return;
      }
      setEditConflict(null);
    },
  });

  function handleSearchSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSearch(searchInput.trim());
    setExplicitInstallationUuid(null);
  }

  function handleBeneficiarySearchSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBeneficiarySearch(beneficiarySearchInput.trim());
    setExplicitBeneficiaryUuid(null);
  }

  function handleAssignSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedInstallation || !selectedBeneficiaryUuid) {
      return;
    }
    assignMutation.mutate({
      installationUuid: selectedInstallation.uuid,
      beneficiaryUuid: selectedBeneficiaryUuid,
      role: assignmentRole,
    });
  }

  function handleCreateContactValueChange(type: 'phone' | 'email', value: string) {
    void value;
    setCreateConflict((current) => (current && current.type === type ? null : current));
    setNewBeneficiary((prev) => {
      if (type === 'phone' && prev.phoneProxyBeneficiaryUuid) {
        return { ...prev, phoneProxyBeneficiaryUuid: '' };
      }
      if (type === 'email' && prev.emailProxyBeneficiaryUuid) {
        return { ...prev, emailProxyBeneficiaryUuid: '' };
      }
      return prev;
    });
  }

  function clearCreateProxy(type: 'phone' | 'email') {
    setNewBeneficiary((prev) => {
      if (type === 'phone' && prev.phoneProxyBeneficiaryUuid) {
        return { ...prev, phoneProxyBeneficiaryUuid: '' };
      }
      if (type === 'email' && prev.emailProxyBeneficiaryUuid) {
        return { ...prev, emailProxyBeneficiaryUuid: '' };
      }
      return prev;
    });
    setCreateConflict((current) => (current && current.type === type ? null : current));
  }

  function handleLinkCreateConflictAsProxy(conflict: ContactConflict) {
    setNewBeneficiary((prev) => {
      const next = { ...prev };
      if (conflict.type === 'phone') {
        next.phoneNumber = conflict.value;
        next.phoneProxyBeneficiaryUuid = conflict.beneficiaryUuid;
      } else {
        next.email = conflict.value;
        next.emailProxyBeneficiaryUuid = conflict.beneficiaryUuid;
      }
      return next;
    });
    setCreateConflict(null);
    setCreateFormError(null);
  }

  function handleCreateConflictDismiss() {
    setCreateConflict((current) => {
      if (!current) {
        return null;
      }
      setNewBeneficiary((prev) => {
        if (current.type === 'phone' && prev.phoneProxyBeneficiaryUuid) {
          return { ...prev, phoneProxyBeneficiaryUuid: '' };
        }
        if (current.type === 'email' && prev.emailProxyBeneficiaryUuid) {
          return { ...prev, emailProxyBeneficiaryUuid: '' };
        }
        return prev;
      });
      return null;
    });
  }

  function handleEditContactValueChange(type: 'phone' | 'email', value: string) {
    void value;
    setEditConflict((current) => (current && current.type === type ? null : current));
    setEditBeneficiary((prev) => {
      if (type === 'phone' && prev.phoneProxyBeneficiaryUuid) {
        return { ...prev, phoneProxyBeneficiaryUuid: '' };
      }
      if (type === 'email' && prev.emailProxyBeneficiaryUuid) {
        return { ...prev, emailProxyBeneficiaryUuid: '' };
      }
      return prev;
    });
  }

  function clearEditProxy(type: 'phone' | 'email') {
    setEditBeneficiary((prev) => {
      if (type === 'phone' && prev.phoneProxyBeneficiaryUuid) {
        return { ...prev, phoneProxyBeneficiaryUuid: '' };
      }
      if (type === 'email' && prev.emailProxyBeneficiaryUuid) {
        return { ...prev, emailProxyBeneficiaryUuid: '' };
      }
      return prev;
    });
    setEditConflict((current) => (current && current.type === type ? null : current));
  }

  function handleLinkEditConflictAsProxy(conflict: ContactConflict) {
    setEditBeneficiary((prev) => {
      const next = { ...prev };
      if (conflict.type === 'phone') {
        next.phoneNumber = conflict.value;
        next.phoneProxyBeneficiaryUuid = conflict.beneficiaryUuid;
      } else {
        next.email = conflict.value;
        next.emailProxyBeneficiaryUuid = conflict.beneficiaryUuid;
      }
      return next;
    });
    setEditConflict(null);
    setEditFormError(null);
  }

  function handleEditConflictDismiss() {
    setEditConflict((current) => {
      if (!current) {
        return null;
      }
      setEditBeneficiary((prev) => {
        if (current.type === 'phone' && prev.phoneProxyBeneficiaryUuid) {
          return { ...prev, phoneProxyBeneficiaryUuid: '' };
        }
        if (current.type === 'email' && prev.emailProxyBeneficiaryUuid) {
          return { ...prev, emailProxyBeneficiaryUuid: '' };
        }
        return prev;
      });
      return null;
    });
  }

  function handleCreateBeneficiarySubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setCreateConflict(null);
    const normalized = normalizeBeneficiaryForm(newBeneficiary);
    if (normalized.error) {
      setCreateFormError(normalized.error);
      return;
    }

    const normalizedSuccess = normalized as NormalizedBeneficiaryFormSuccess;

    const payload: CreateBeneficiaryPayload = {
      name: normalizedSuccess.name,
      email: normalizedSuccess.email,
      phoneNumber: normalizedSuccess.phoneNumber,
      address: normalizedSuccess.address,
    };

    if (normalizedSuccess.contacts.length) {
      payload.contacts = normalizedSuccess.contacts;
    }
    if (normalizedSuccess.location) {
      payload.location = normalizedSuccess.location;
    }

    setCreateFormError(null);
    createBeneficiaryMutation.mutate(payload);
  }

  function handleStartEditBeneficiary(beneficiary: Beneficiary) {
    setEditFormError(null);
    setEditConflict(null);
    setCreateConflict(null);
    setEditingBeneficiary(beneficiary);
    setEditBeneficiary(populateFormFromBeneficiary(beneficiary));
    setShowCreateBeneficiaryForm(false);
    setExplicitBeneficiaryUuid(beneficiary.uuid);
  }

  function handleEditBeneficiarySubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!editingBeneficiary) {
      return;
    }

    setEditConflict(null);
    const normalized = normalizeBeneficiaryForm(editBeneficiary);
    if (normalized.error) {
      setEditFormError(normalized.error);
      return;
    }

    const normalizedSuccess = normalized as NormalizedBeneficiaryFormSuccess;
    const mergedContacts = mergeContactsForUpdate(editingBeneficiary, normalizedSuccess.contacts);

    const payload: UpdateBeneficiaryPayload = {
      name: normalizedSuccess.name,
      email: normalizedSuccess.email,
      phoneNumber: normalizedSuccess.phoneNumber,
      address: normalizedSuccess.address,
      contacts: mergedContacts,
      location: normalizedSuccess.location ?? null,
    };

    setEditFormError(null);
    updateBeneficiaryMutation.mutate({
      beneficiaryUuid: editingBeneficiary.uuid,
      payload,
    });
  }

  const assignError = assignMutation.error as Error | null;
  const createError = createBeneficiaryMutation.error ?? null;
  const removeError = removeAssignmentMutation.error as Error | null;
  const updateError = updateBeneficiaryMutation.error ?? null;
  const canManageBeneficiaries =
    hasCapability('admin:all') || hasCapability('beneficiaries:manage');
  const canRemoveAssignments =
    hasCapability(['installations:manage', 'beneficiaries:manage'], { match: 'all' }) ||
    hasCapability('admin:all');

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h2 className="text-2xl font-semibold text-slate-800">Installations &amp; Beneficiaries</h2>
        <p className="text-sm text-slate-600">
          Review enrolled pumps, update beneficiary assignments, and invite new beneficiaries when
          needed.
        </p>
      </div>

      <div className="grid gap-6 lg:grid-cols-[1.6fr_1fr]">
        <section className="flex flex-col gap-4 rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <h3 className="text-lg font-semibold text-slate-800">Installations</h3>
              <p className="text-sm text-slate-500">
                Search by IMEI, UUID, or beneficiary keyword.
              </p>
            </div>
            <form onSubmit={handleSearchSubmit} className="flex w-full max-w-sm gap-2">
              <input
                type="search"
                value={searchInput}
                onChange={(event) => setSearchInput(event.target.value)}
                placeholder="Search installations"
                className="flex-1 rounded border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-200"
              />
              <button
                type="submit"
                className="rounded bg-emerald-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-60"
                disabled={installationsQuery.isLoading}
              >
                {installationsQuery.isLoading ? 'Searching…' : 'Search'}
              </button>
            </form>
          </div>

          {installationsQuery.isError && (
            <div className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
              {(installationsQuery.error as Error).message}
            </div>
          )}

          <div className="overflow-hidden rounded border border-slate-200">
            <table className="min-w-full divide-y divide-slate-200 text-sm">
              <thead className="bg-slate-50 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
                <tr>
                  <th className="px-3 py-2">IMEI</th>
                  <th className="px-3 py-2">Status</th>
                  <th className="px-3 py-2">Beneficiaries</th>
                  <th className="px-3 py-2">Updated</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200">
                {installations.map((installation) => {
                  const isSelected = installation.uuid === selectedInstallationUuid;
                  return (
                    <tr
                      key={installation.uuid}
                      className={`${
                        isSelected ? 'bg-emerald-50/70' : 'bg-white'
                      } transition-colors hover:bg-emerald-50`}
                    >
                      <td className="px-3 py-2">
                        <button
                          type="button"
                          onClick={() => setExplicitInstallationUuid(installation.uuid)}
                          className="text-left text-sm font-medium text-emerald-700 hover:underline"
                        >
                          {installation.imei}
                        </button>
                        <div className="text-xs text-slate-500">{installation.deviceUuid}</div>
                      </td>
                      <td className="px-3 py-2">
                        <StatusBadge status={installation.status} />
                      </td>
                      <td className="px-3 py-2">
                        {installation.beneficiaryCount}
                        <span className="text-xs text-slate-500"> active</span>
                      </td>
                      <td className="px-3 py-2 text-xs text-slate-500">
                        {formatDate(installation.updatedAt)}
                      </td>
                    </tr>
                  );
                })}
                {!installations.length && (
                  <tr>
                    <td className="px-3 py-6 text-center text-sm text-slate-500" colSpan={4}>
                      {installationsQuery.isLoading
                        ? 'Loading installations…'
                        : 'No installations found for the selected filters.'}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </section>

        <section className="flex flex-col gap-4 rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <div className="flex items-center justify-between">
            <h3 className="text-lg font-semibold text-slate-800">Installation details</h3>
          </div>

          {!selectedInstallation && (
            <p className="text-sm text-slate-500">
              Select an installation to review beneficiary assignments and metadata.
            </p>
          )}

          {selectedInstallation && (
            <div className="flex flex-col gap-4">
              <div className="grid gap-2 text-sm text-slate-600">
                <div>
                  <span className="font-medium text-slate-700">IMEI:</span>{' '}
                  {selectedInstallation.imei}
                </div>
                <div>
                  <span className="font-medium text-slate-700">Installation UUID:</span>{' '}
                  {selectedInstallation.uuid}
                </div>
                <div>
                  <span className="font-medium text-slate-700">Status:</span>{' '}
                  <StatusBadge status={selectedInstallation.status} />
                </div>
                <div>
                  <span className="font-medium text-slate-700">Activated:</span>{' '}
                  {formatDate(selectedInstallation.activatedAt)}
                </div>
                <div>
                  <span className="font-medium text-slate-700">Last updated:</span>{' '}
                  {formatDate(selectedInstallation.updatedAt)}
                </div>
                {selectedInstallation.geoLocation && (
                  <div>
                    <span className="font-medium text-slate-700">Geo location:</span>{' '}
                    {selectedInstallation.geoLocation.latitude},{' '}
                    {selectedInstallation.geoLocation.longitude}
                  </div>
                )}
                {selectedInstallation.notes && (
                  <div>
                    <span className="font-medium text-slate-700">Notes:</span>{' '}
                    {selectedInstallation.notes}
                  </div>
                )}
              </div>

              {selectedInstallation.metadata && (
                <details className="rounded border border-slate-200 bg-slate-50 px-3 py-2 text-xs text-slate-600">
                  <summary className="cursor-pointer text-sm font-medium text-slate-700">
                    Metadata
                  </summary>
                  <pre className="mt-2 whitespace-pre-wrap break-all">
                    {JSON.stringify(selectedInstallation.metadata, null, 2)}
                  </pre>
                </details>
              )}

              <div className="flex flex-col gap-3">
                <div className="flex items-center justify-between">
                  <h4 className="text-base font-semibold text-slate-800">
                    Beneficiary assignments
                  </h4>
                  {assignmentsQuery.isFetching && (
                    <span className="text-xs text-slate-500">Refreshing…</span>
                  )}
                </div>

                {assignmentsQuery.isError && (
                  <div className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                    {(assignmentsQuery.error as Error).message}
                  </div>
                )}

                {!canRemoveAssignments && (
                  <div className="rounded border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700">
                    Beneficiary removal requires both installations:manage and beneficiaries:manage
                    capabilities or super-admin access.
                  </div>
                )}

                {removeError && (
                  <div className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                    {removeError.message}
                  </div>
                )}

                <div className="flex flex-col gap-3">
                  {assignments.map((assignment) => {
                    const contacts = getDisplayContacts(assignment.beneficiary);
                    const isBeingEdited = editingBeneficiary?.uuid === assignment.beneficiaryUuid;
                    const cardClassName = `flex flex-col justify-between gap-2 rounded border p-3 text-sm text-slate-600 sm:flex-row sm:items-center ${
                      isBeingEdited
                        ? 'border-emerald-300 bg-emerald-50/70'
                        : 'border-slate-200 bg-slate-50'
                    }`;
                    return (
                      <div key={assignment.id} className={cardClassName}>
                        <div className="flex flex-col gap-1">
                          <span className="font-medium text-slate-800">
                            {assignment.beneficiary.name}
                          </span>
                          <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500">
                            <StatusBadge status={assignment.role} label={assignment.role} />
                            <StatusBadge
                              status={assignment.assignmentStatus}
                              label={assignment.assignmentStatus}
                            />
                            {contacts.map((contact) => (
                              <span
                                key={contact.key}
                                className={`inline-flex items-center gap-1 rounded-full border border-slate-300 px-2 py-1 ${
                                  contact.isPrimary ? 'bg-white text-slate-600' : 'bg-slate-100'
                                }`}
                              >
                                {contact.label}
                              </span>
                            ))}
                          </div>
                        </div>
                        <div className="flex items-center gap-2 text-xs">
                          <span className="text-slate-500">
                            Assigned {formatDate(assignment.createdAt)}
                          </span>
                          {canManageBeneficiaries && (
                            <button
                              type="button"
                              onClick={() => handleStartEditBeneficiary(assignment.beneficiary)}
                              className="rounded border border-emerald-300 px-2 py-1 text-xs font-medium text-emerald-700 transition-colors hover:bg-emerald-50 disabled:cursor-not-allowed disabled:opacity-60"
                              disabled={updateBeneficiaryMutation.isPending}
                            >
                              {isBeingEdited ? 'Editing…' : 'Edit'}
                            </button>
                          )}
                          {assignment.assignmentStatus === 'active' && (
                            <button
                              type="button"
                              onClick={() => {
                                if (!canRemoveAssignments) {
                                  return;
                                }
                                removeAssignmentMutation.mutate({
                                  installationUuid: assignment.installationUuid,
                                  beneficiaryUuid: assignment.beneficiaryUuid,
                                });
                              }}
                              className="rounded bg-red-600 px-2 py-1 text-xs font-medium text-white transition-colors hover:bg-red-500 disabled:cursor-not-allowed disabled:opacity-60"
                              disabled={!canRemoveAssignments || removeAssignmentMutation.isPending}
                              title={
                                !canRemoveAssignments
                                  ? 'Installations:manage and beneficiaries:manage capabilities or super-admin access required to remove assignments'
                                  : undefined
                              }
                            >
                              {removeAssignmentMutation.isPending ? 'Removing…' : 'Remove'}
                            </button>
                          )}
                        </div>
                      </div>
                    );
                  })}

                  {!assignments.length && !assignmentsQuery.isLoading && (
                    <p className="rounded border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-500">
                      No beneficiaries assigned yet.
                    </p>
                  )}
                </div>

                <form
                  className="flex flex-col gap-3 rounded border border-emerald-200 bg-emerald-50 p-3"
                  onSubmit={handleAssignSubmit}
                >
                  <label className="flex flex-col gap-2 text-xs font-semibold uppercase tracking-wide text-emerald-700">
                    <span>Beneficiary</span>
                    <select
                      value={selectedBeneficiaryUuid ?? ''}
                      onChange={(event: ChangeEvent<HTMLSelectElement>) =>
                        setExplicitBeneficiaryUuid(event.target.value || null)
                      }
                      className="rounded border border-emerald-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-200"
                    >
                      {availableBeneficiaries.map((beneficiary: Beneficiary) => (
                        <option key={beneficiary.uuid} value={beneficiary.uuid}>
                          {beneficiary.name}
                        </option>
                      ))}
                      {!availableBeneficiaries.length && (
                        <option value="">No available beneficiaries</option>
                      )}
                    </select>
                  </label>

                  <label className="flex flex-col gap-2 text-xs font-semibold uppercase tracking-wide text-emerald-700">
                    <span>Assignment role</span>
                    <select
                      value={assignmentRole}
                      onChange={(event: ChangeEvent<HTMLSelectElement>) =>
                        setAssignmentRole(event.target.value as 'owner' | 'secondary')
                      }
                      className="rounded border border-emerald-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-200"
                    >
                      <option value="owner">Owner</option>
                      <option value="secondary">Secondary</option>
                    </select>
                  </label>

                  {assignError && (
                    <div className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                      {assignError.message}
                    </div>
                  )}

                  <button
                    type="submit"
                    className="self-start rounded bg-emerald-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-60"
                    disabled={
                      !selectedBeneficiaryUuid ||
                      assignMutation.isPending ||
                      !availableBeneficiaries.length
                    }
                  >
                    {assignMutation.isPending ? 'Assigning…' : 'Assign beneficiary'}
                  </button>
                </form>

                <div className="flex flex-col gap-3 rounded border border-slate-200 bg-slate-50 p-3">
                  <div className="flex items-center justify-between">
                    <h5 className="text-sm font-semibold text-slate-800">Find beneficiaries</h5>
                    <button
                      type="button"
                      className="text-xs font-medium text-emerald-700 hover:underline"
                      onClick={() => {
                        setCreateFormError(null);
                        setCreateConflict(null);
                        setEditingBeneficiary(null);
                        setEditConflict(null);
                        setEditBeneficiary(makeEmptyBeneficiaryForm());
                        setShowCreateBeneficiaryForm((value) => !value);
                      }}
                      disabled={Boolean(editingBeneficiary) || updateBeneficiaryMutation.isPending}
                      title={
                        editingBeneficiary
                          ? 'Finish editing the current beneficiary before creating a new one'
                          : undefined
                      }
                    >
                      {showCreateBeneficiaryForm ? 'Hide form' : 'Create beneficiary'}
                    </button>
                  </div>

                  <form onSubmit={handleBeneficiarySearchSubmit} className="flex gap-2">
                    <input
                      type="search"
                      value={beneficiarySearchInput}
                      onChange={(event) => setBeneficiarySearchInput(event.target.value)}
                      placeholder="Search beneficiaries"
                      className="flex-1 rounded border border-slate-300 px-3 py-2 text-sm focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-200"
                    />
                    <button
                      type="submit"
                      className="rounded bg-emerald-600 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-60"
                      disabled={beneficiariesQuery.isLoading}
                    >
                      {beneficiariesQuery.isLoading ? 'Searching…' : 'Search'}
                    </button>
                  </form>

                  {beneficiariesQuery.isError && (
                    <div className="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                      {(beneficiariesQuery.error as Error).message}
                    </div>
                  )}

                  <div className="flex flex-col gap-2 text-xs text-slate-500">
                    <span>
                      Showing {availableBeneficiaries.length} available beneficiary options.
                    </span>
                    {beneficiariesQuery.isLoading && <span>Loading directory…</span>}
                  </div>

                  {editingBeneficiary && (
                    <BeneficiaryForm
                      title={`Edit beneficiary: ${editingBeneficiary.name}`}
                      formState={editBeneficiary}
                      setFormState={setEditBeneficiary}
                      onSubmit={handleEditBeneficiarySubmit}
                      submitting={updateBeneficiaryMutation.isPending}
                      submitLabel="Save changes"
                      submittingLabel="Saving…"
                      clientError={editFormError}
                      serverError={editConflict ? null : updateError}
                      helperText="Updating contact or location details will replace the primary entries."
                      onCancel={() => {
                        setEditingBeneficiary(null);
                        setEditFormError(null);
                        setEditConflict(null);
                        setEditBeneficiary(makeEmptyBeneficiaryForm());
                      }}
                      cancelLabel="Cancel"
                      conflict={editConflict}
                      onLinkConflictAsProxy={handleLinkEditConflictAsProxy}
                      onConflictDismiss={handleEditConflictDismiss}
                      onContactValueChange={handleEditContactValueChange}
                      onClearProxy={clearEditProxy}
                    />
                  )}

                  {showCreateBeneficiaryForm && (
                    <BeneficiaryForm
                      title="Create beneficiary"
                      formState={newBeneficiary}
                      setFormState={setNewBeneficiary}
                      onSubmit={handleCreateBeneficiarySubmit}
                      submitting={createBeneficiaryMutation.isPending}
                      submitLabel="Save beneficiary"
                      submittingLabel="Creating…"
                      clientError={createFormError}
                      serverError={createConflict ? null : createError}
                      helperText="Optional: record administrative location and geo coordinates if available."
                      conflict={createConflict}
                      onLinkConflictAsProxy={handleLinkCreateConflictAsProxy}
                      onConflictDismiss={handleCreateConflictDismiss}
                      onContactValueChange={handleCreateContactValueChange}
                      onClearProxy={clearCreateProxy}
                    />
                  )}
                </div>
              </div>
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
