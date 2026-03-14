import { VendorsSection } from '../features/admin/vendors/VendorsSection';

export function AdminDriveManufacturersPage() {
  return (
    <div className="space-y-6">
      <VendorsSection initialCollection="vfdManufacturer" collections={['vfdManufacturer']} />
    </div>
  );
}
