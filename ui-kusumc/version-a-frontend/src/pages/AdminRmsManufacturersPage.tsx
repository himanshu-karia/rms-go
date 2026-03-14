import { VendorsSection } from '../features/admin/vendors/VendorsSection';

export function AdminRmsManufacturersPage() {
  return (
    <div className="space-y-6">
      <VendorsSection initialCollection="rmsManufacturer" collections={['rmsManufacturer']} />
    </div>
  );
}
